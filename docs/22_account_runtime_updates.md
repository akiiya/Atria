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
│           + AccountGate (per-account mutex)            │
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
  │  TelegramChannelUpdateState     │
  └─────────────────────────────────┘
```

## Runtime 启动策略

**不默认启动所有账号 runtime**。原因：
- 避免离线部署启动时就大量连接 Telegram
- 避免无意义的连接消耗
- 用户可能有多个账号但只使用一个

**启动时机**：
1. 用户进入 `/app/#/chats` 页面时
2. 前端调用 `GET /api/chats/runtime/status` 查询状态
3. 如果 state 是 `stopped`，自动调用 `POST /api/chats/runtime/start`
4. 只启动当前 selected account 的 runtime

**幂等性**：
- 如果 runtime 已启动（live/connecting/syncing），`StartAccount` 返回 nil
- 如果 runtime 是 stopped/degraded/offline，清理后重新启动

## Runtime API

### GET /api/chats/runtime/status

返回当前 selected account 的 runtime 状态。

```json
{
  "ok": true,
  "account_id": 1,
  "state": "live",
  "last_sync_at": "2026-06-11T16:00:00Z",
  "last_event_at": "2026-06-11T16:05:00Z",
  "last_error": "",
  "active": true
}
```

### POST /api/chats/runtime/start

启动当前 selected account 的 runtime。

```json
{
  "ok": true,
  "account_id": 1,
  "state": "connecting"
}
```

### POST /api/chats/runtime/stop

停止当前 selected account 的 runtime。

```json
{
  "ok": true,
  "account_id": 1,
  "state": "stopped"
}
```

**安全约束**：
- 必须鉴权
- 必须 CSRF 保护（POST 请求）
- 使用当前 selected account，不允许操作不可见账号
- 不返回 api_hash、proxy_password、session path、access_hash
- last_error 脱敏

## REST 与 Runtime 并发边界

### 问题

同一 account 的 REST 临时 gotd client 和 Runtime long-lived gotd client 可能并发运行。仅靠 `FileBackedSessionStorage` 的文件锁不够：
- 文件锁只能降低 session 文件损坏风险
- 不等于协议状态、updates state、连接生命周期完全安全

### 解决方案：Per-Account Execution Gate

采用方案 B（execution gate）作为过渡方案：

1. `AccountGate` 管理 per-account 的 `sync.Mutex`
2. Runtime 启动时持有该 account 的 gate lock
3. REST adapter 执行前获取同一 account 的 gate lock
4. 同一 account 不会同时运行多个 gotd client

```go
// Runtime 持有锁
m.gate.Lock(rt.accountID, "runtime")
defer m.gate.Unlock(rt.accountID)

// REST 获取锁
unlock := a.acquireGate(req.AccountID)
defer unlock()
```

**这是过渡方案**。已升级为 Runtime Execution Queue（见下文）。

## Runtime Execution Queue

### 设计

每个 `AccountRuntime` 持有一个 `RuntimeExecutor`，提供串行 execution queue：

```go
type RuntimeExecutor struct {
    requestCh chan executeRequest  // buffer 64
    doneCh    chan struct{}        // 关闭信号
}

type ExecuteFunc func(ctx context.Context, api *tg.Client) error
```

### 行为

1. **串行执行**：queue 中的请求按顺序执行，避免 gotd API 并发不确定性
2. **context cancellation**：支持 context 取消和超时
3. **timeout**：默认 30s，可通过 context deadline 覆盖
4. **queue full**：buffer 满时返回 `ErrorCodeRuntimeQueueFull`
5. **panic recover**：executor 捕获 panic 并返回错误
6. **stop drain**：runtime 停止时，等待中的请求全部返回 `ErrorCodeRuntimeStopped`

### REST 请求路由

```
REST request
→ ChatService
→ telegramclient.ClientAdapter
→ gotd Adapter
→ 检查 runtime executor 是否可用
  → 可用：通过 executor.Execute() 使用 runtime client
  → 不可用：fallback 到临时 client + AccountGate
```

### 状态与 Execute 关系

| Runtime 状态 | Execute 行为 |
|-------------|-------------|
| `live` | 通过 executor 执行 |
| `syncing` | 通过 executor 执行（client API 已 ready） |
| `connecting` | 通过 executor 执行（等待 ready） |
| `stopped` | 返回 `runtime_stopped`，adapter fallback |
| `degraded` | 返回错误，adapter fallback |
| `offline` | 返回错误，adapter fallback |

### AccountGate 角色调整

- **不再**作为 runtime live 时阻塞 REST 的主方案
- **只用于** temporary client fallback
- Runtime executor 自行保证串行
- Fallback 时仍通过 AccountGate 防止并发

### 新增错误码

| 错误码 | 说明 |
|--------|------|
| `runtime_not_ready` | Runtime 未就绪 |
| `runtime_stopped` | Runtime 已停止 |
| `runtime_queue_full` | 请求队列已满 |
| `runtime_execute_timeout` | 执行超时 |

## Channel Update State 持久化

### TelegramChannelUpdateState 模型

```go
type TelegramChannelUpdateState struct {
    ID         uint
    AccountID  uint       `gorm:"uniqueIndex:idx_channel_state"`
    ChannelID  int64      `gorm:"uniqueIndex:idx_channel_state"`
    Pts        int
    LastSyncAt *time.Time
}
```

### 实现

- `GetChannelPts(accountID, channelID)` → 查询 `TelegramChannelUpdateState`
- `SetChannelPts(accountID, channelID, pts)` → upsert 到 `TelegramChannelUpdateState`
- `ForEachChannels(accountID, f)` → 遍历所有频道 state

不再返回空值或跳过操作。

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

## Update Handler 硬化

### new message
- 写入 `ChatMessageCache`（AES-256-GCM 加密）
- 更新 `ChatPeerCache` preview 和 last_message_at
- 发布 `EventMessageNew`
- 更新 runtime `lastEvent` 时间

### new channel message
- 同 new message，但 peer_ref 格式为 `ch_<id>`

### edit message
- 更新 `ChatMessageCache` text/sender_name/kind
- 发布 `EventMessageEdited`

### delete message
- 从 `ChatMessageCache` 删除
- 发布 `EventMessageDeleted`

### unsupported update
- 安全忽略
- debug 日志只记录 update type
- 不记录 body

## 安全策略

- 不记录 message body 到日志（只记录 text_len）
- 不记录 api_hash、proxy_password、session path、access_hash
- UpdateEvent 日志只记录 event type、account_id、peer_ref、message_id
- ChatMessageCache 使用 AES-256-GCM 加密正文
- TelegramUpdateState 不存储敏感字段
- TelegramChannelUpdateState 不存储敏感字段
- Runtime status API 不返回敏感信息

## 前端状态接入

ChatView 在加载时：
1. 查询 `GET /api/chats/runtime/status`
2. 如果 state 是 `stopped`，自动调用 `POST /api/chats/runtime/start`
3. 在 sidebar header 显示轻量状态指示器
4. 每 60 秒低频刷新 status
5. 不影响现有消息加载

状态显示：
- `connecting`：正在连接
- `syncing`：正在同步
- `live`：实时更新中
- `degraded`：同步异常
- `offline`：连接断开
- `stopped`：未启动

## 真实 Updates 手动验证步骤

1. 启动 `bin/atria.exe serve`
2. 登录 Atria
3. 选择已登录 Telegram 账号
4. 打开 `/app/#/chats`
5. 确认 runtime status 从 `stopped` → `connecting` → `syncing` → `live`
6. 用手机 Telegram 或官方客户端给该账号发送一条消息
7. 不刷新页面
8. 检查 `GET /api/chats/runtime/status` 的 `last_event_at` 是否更新
9. 检查 `ChatMessageCache` 是否新增消息（通过再次加载 chats 页面验证）
10. 检查 `ChatPeerCache` preview 是否更新
11. 再刷新 chats 页面，确认新消息来自 cache
12. 停止 runtime，再启动，确认 state 不丢（检查 `TelegramUpdateState` 表）
13. 断网/代理失败时，确认 state 进入 `degraded`/`offline`，且不泄露敏感日志

**注意**：本轮没有 WebSocket，所以"不刷新页面自动出现消息"不要求。但 cache 和 runtime status 必须能证明后端 updates 已工作。

## Runtime Execution Queue 手动验证

1. 启动 `bin/atria.exe serve`
2. 打开 `/app/#/chats`
3. 确认 runtime status 进入 `connecting` → `syncing` → `live`
4. 点击会话，确认 dialogs/messages 正常
5. 查看日志，确认 REST 请求**不**出现 "使用临时 client fallback"（说明走了 executor）
6. 发送一条文本消息，确认成功
7. 向上加载 older，确认成功
8. 停止 runtime（通过 API 或重启服务）
9. 再请求 messages，确认出现 "使用临时 client fallback"（说明 fallback 正常）
10. 重启 runtime，确认 REST 又走 executor
11. 用官方 Telegram 给该账号发消息，确认 runtime updates 仍能写 cache
12. 全程不应出现 session 冲突、死锁、长期 pending

**日志关键词**：
- `runtime_queue`：通过 executor 执行（正常路径）
- `使用临时 client fallback`：fallback 到临时 client（runtime 不可用时）

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
- AccountGate 可以复用

## 离线恢复 getDifference

当 runtime 重启时：
1. 从 `TelegramUpdateState` 读取上次的 pts/qts/date/seq
2. 从 `TelegramChannelUpdateState` 读取频道 pts
3. `updates.Manager.Run()` 自动调用 `updates.getDifference`
4. 恢复期间状态为 `syncing`
5. 恢复完成后状态变为 `live`

## 为什么本轮不做 WebSocket

- WebSocket 需要前端配合（连接管理、重连、消息格式）
- 后端 EventBus 已经就绪，下一轮直接接入
- 本轮专注于后端基础的正确性和测试覆盖

## 为什么不做全量历史同步

- 全量同步会消耗大量 Telegram API 配额
- 可能触发 FLOOD_WAIT
- Atria 定位是轻量客户端，不是归档工具
- cache-first + 按需加载更符合使用场景

## EventBus 与 WebSocket

EventBus 现在接入 WebSocket 实时推送：

- WebSocket endpoint: `GET /api/realtime/ws`
- 只订阅当前 selected account 的 EventBus
- 推送中立 UpdateEvent（不包含 gotd 类型）
- 前端收到事件后局部 patch TanStack Query cache
- 详见 `docs/23_websocket_realtime_push.md`

Runtime Execution Queue 与 WebSocket 的关系：
- REST 请求通过 runtime executor 执行
- WebSocket 只消费 EventBus 事件
- 两者独立，互不阻塞

本轮仍不做全量同步，只做实时事件推送。
