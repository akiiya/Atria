# AccountRuntime + gotd Updates 架构

## 为什么引入 AccountRuntime

当前 Atria 的聊天是"请求式"的：每个 REST API 调用创建一个临时 gotd client，执行一次 RPC，然后销毁。这意味着：
- 没有持久连接
- 没有实时更新
- 每次请求都要重新建立 MTProto 连接
- 无法接收 Telegram 推送的新消息

AccountRuntime 为每个 active Telegram account 维护一个长-lived 的 gotd client，配合 `telegram/updates` Manager 处理 Telegram 的实时更新推送。

## 架构概述

```
┌─────────────────────────────────────────────────────┐
│                  RuntimeManagerImpl                   │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │AccountRuntime│  │AccountRuntime│  │  EventBus   │ │
│  │  account 1   │  │  account 2   │  │             │ │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │  subscribe  │ │
│  │ │gotd client│ │  │ │gotd client│ │  │  publish   │ │
│  │ │  + updates│ │  │ │  + updates│ │  │             │ │
│  │ │  Manager  │ │  │ │  Manager  │ │  └────────────┘ │
│  │ └──────────┘ │  │ └──────────┘ │                   │
│  └──────────────┘  └──────────────┘                   │
└─────────────────────────────────────────────────────┘
         │                    │
         ▼                    ▼
  ┌─────────────┐     ┌─────────────┐
  │UpdateHandler│     │UpdateHandler│
  │  map → neutral   │  map → neutral
  │  write cache│     │  write cache│
  │  publish    │     │  publish    │
  └─────────────┘     └─────────────┘
         │                    │
         ▼                    ▼
  ┌─────────────────────────────────┐
  │        SQLite Cache              │
  │  ChatMessageCache (encrypted)   │
  │  ChatPeerCache                  │
  │  TelegramUpdateState            │
  └─────────────────────────────────┘
```

## 每个 Account 一个 Runtime

- `RuntimeManagerImpl` 管理多个 `AccountRuntime`
- 每个 `AccountRuntime` 持有一个 `telegram.Client` + `updates.Manager`
- 同一 account 不会创建多个 runtime（幂等 start）
- Stop 时取消 context，client 和 updates Manager 一起关闭

## gotd Updates 处理

使用 `github.com/gotd/td/telegram/updates` 包：

1. 创建 `updates.Manager`，传入：
   - `Handler`：我们的 `UpdateHandler` 实现
   - `Storage`：我们的 `StateStore`（SQLite 持久化）
   - `AccessHasher`：我们的 `HashStore`（ChatPeerCache）

2. 将 `updates.Manager` 作为 `UpdateHandler` 设置到 `telegram.Options`

3. 在 `client.Run()` 回调中调用 `updatesManager.Run(ctx, client.API(), userID, opts)`

4. `updates.Manager` 自动处理：
   - pts/qts/seq 排序
   - gap 检测
   - `getDifference` 恢复
   - 将有序 update 转发给我们的 `UpdateHandler`

## Update State 持久化

`TelegramUpdateState` 模型存储：
- `AccountID`（唯一索引）
- `Pts`、`Qts`、`Date`、`Seq`
- `LastSyncAt`

按 account_id 隔离。不存储敏感字段（access_hash、api_hash、session path、proxy_password）。

离线恢复时，`updates.Manager` 会：
1. 从 `StateStorage` 读取上次的 state
2. 如果 state 存在，使用 `getDifference` 恢复遗漏的更新
3. 如果 state 不存在，从 Telegram 获取最新 state

## 内部 EventBus

`EventBus` 提供 per-account 的事件订阅和发布：

- `Subscribe(accountID, sink)` → 返回 `Subscription`
- `Publish(accountID, event)` → 非阻塞，channel 满时丢弃
- `Unsubscribe(accountID)` → 移除所有订阅
- `Subscription.Close()` → 取消单个订阅

关键设计：
- 每个 subscriber 有独立的 buffered channel（100 events）
- Publish 是非阻塞的，慢 subscriber 不阻塞 runtime
- 下一轮 WebSocket 会接这个 EventBus

## UpdateEvent 类型

所有事件使用 `telegramclient.UpdateEvent`，不包含 gotd 类型：

| 事件类型 | 说明 | Payload |
|---------|------|---------|
| `message.new` | 新消息 | `telegramclient.Message` |
| `message.edited` | 编辑消息 | `telegramclient.Message` |
| `message.deleted` | 删除消息 | `map[string]any{"message_ids": []}` |
| `dialog.upserted` | 会话更新 | `telegramclient.Dialog` |
| `account.connected` | 连接成功 | nil |
| `account.disconnected` | 断开连接 | nil |
| `sync.done` | 同步完成 | nil |

## REST 与 Runtime 共存

- REST API 继续使用临时 gotd client（不变）
- Runtime 使用长-lived client
- 两者可能同时访问同一 session 文件
- gotd 的 `FileBackedSessionStorage` 使用文件级锁，安全

## 安全策略

- 不记录 message body 到日志（只记录 text_len）
- 不记录 api_hash、proxy_password、session path、access_hash
- UpdateEvent 日志只记录 event type、account_id、peer_ref、message_id
- ChatMessageCache 使用 AES-256-GCM 加密正文
- TelegramUpdateState 不存储敏感字段

## WebSocket 下一轮

本轮只建立后端基础。下一轮会：
1. 创建 WebSocket endpoint
2. WebSocket handler 订阅 EventBus
3. 将 UpdateEvent 推送到前端
4. 前端实时更新消息列表和会话列表

## TDLib 替换路径

未来切 TDLib 时：
- `internal/telegramclient/tdlib/runtime.go` 实现 `RuntimeManager`
- TDLib 的 update handler 映射为同样的 `UpdateEvent`
- 上层代码（chat service、server handler）不需要修改
- EventBus、StateStore 接口可以复用

## 离线恢复 getDifference

当 runtime 重启时：
1. 从 `TelegramUpdateState` 读取上次的 pts/qts/date/seq
2. `updates.Manager.Run()` 自动调用 `updates.getDifference`
3. 恢复期间状态为 `syncing`
4. 恢复完成后状态变为 `live`

当前限制：
- Channel state 持久化尚未完整实现（`GetChannelPts`/`SetChannelPts` 为空操作）
- 当 channel gap 太长时，`updates.Manager` 无法自动恢复
- 后续可在 `ChatPeerCache` 中添加 `channel_pts` 字段

## 为什么本轮不做 WebSocket

- WebSocket 需要前端配合（连接管理、重连、消息格式）
- 后端 EventBus 已经就绪，下一轮直接接入
- 本轮专注于后端基础的正确性和测试覆盖

## 为什么不做全量历史同步

- 全量同步会消耗大量 Telegram API 配额
- 可能触发 FLOOD_WAIT
- Atria 定位是轻量客户端，不是归档工具
- cache-first + 按需加载更符合使用场景
