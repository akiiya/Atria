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
- WebSocket 已接入这个 EventBus（见 `docs/23_websocket_realtime_push.md`）

## UpdateEvent 类型

所有事件使用 `telegramclient.UpdateEvent`，不包含 gotd 类型：

| 事件类型 | 说明 | Payload |
|---------|------|---------|
| `message.new` | 新消息 | `telegramclient.Message` |
| `message.edited` | 编辑消息 | `telegramclient.Message` |
| `message.deleted` | 删除消息 | `map[string]any{"telegram_message_ids": []}` |
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

**历史注意**：该段记录的是 AccountRuntime 初始落地时的验收口径；WebSocket 已在后续阶段接入，当前实时验收以 `docs/23_websocket_realtime_push.md` 为准。

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

## WebSocket 接入状态

当前 WebSocket 已接入：
1. `GET /api/realtime/ws`
2. WebSocket handler 订阅 selected account 的 EventBus
3. 将中立 `UpdateEvent` 推送到前端
4. 前端局部 patch 消息列表和会话列表

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

## 历史说明：最初为什么未在 AccountRuntime 阶段直接做 WebSocket

- WebSocket 需要前端配合（连接管理、重连、消息格式）
- 后端 EventBus 当时已经就绪，后续阶段已接入 WebSocket
- AccountRuntime 阶段专注于后端基础的正确性和测试覆盖

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

### 链路排查指南

**Runtime 正常但 WS 没收到事件：**
1. 检查 `GET /api/chats/runtime/status` 确认 `state=live`
2. 检查 runtime 日志是否有 `新消息处理完成` 或 `消息编辑处理完成`
3. 如果 runtime 没收到 update，检查 `TelegramUpdateState` 表的 pts/qts 是否在变化
4. 如果 runtime 收到了但 WS 没推送，检查 EventBus subscriber count

**WS 收到但 UI 不更新：**
1. 打开浏览器 DevTools WS 面板，确认收到 JSON 事件
2. 检查 `event.account_id` 是否等于当前 `accountId`
3. 检查 `event.peer_ref` 是否等于当前打开的 peer
4. 检查 TanStack Query DevTools 中 `['messages', accountId, peerRef]` 是否被 patch
5. 如果 patch 了但 UI 没更新，检查 Vue 组件是否正确响应式

**关键字段用于排查：**
- `last_event_at`：最后收到事件的时间
- `last_event_type`：最后事件类型
- `last_event_peer_ref`：最后事件的 peer_ref
- `ws_clients_count`：当前 WebSocket 连接数（可选）

## 默认入口 canonical 化

登录后默认入口已 canonical 化到 `/app/#/dashboard`：
- `GET /` 已认证用户 → 重定向到 `/app/#/dashboard`
- 登录成功 → 重定向到 `/app/#/dashboard`
- 初始化成功 → 重定向到 `/app/#/dashboard`
- 旧 `/dashboard`、`/accounts`、`/chats` 路由 → 重定向到 `/app/#/...`
- `/login`、`/init`、`/api/*`、`/healthz` 不受影响

## 2026-06 EventBus to WebSocket troubleshooting update

- Runtime healthy but WebSocket receives no events: check `GET /api/chats/runtime/status` for `state=live`, `last_event_at`, and `last_event_type`; then check runtime logs for neutral `UpdateEvent` publish and EventBus subscriber registration.
- WebSocket receives events but UI does not update: verify `event.account_id` equals the selected account, `event.peer_ref` equals the current peer when patching messages, and TanStack Query has `['messages', accountId, peerRef]` or `['dialogs', accountId]` cached.
- `message.deleted` events must publish `payload.telegram_message_ids`; `message_ids` is only a compatibility input fallback.
- EventBus payloads must remain neutral and must not contain gotd raw types, access hashes, session paths, API hashes, proxy passwords, complete phone numbers, or message body logs.
- WebSocket is now implemented in `docs/23_websocket_realtime_push.md`; the older “next round WebSocket” note is historical.

## 2026-06 REST loading deadlock fix

- `GetExecutor()` returns executor only in `live`/`syncing` states. During `connecting`, the executor's `Run()` goroutine has not started, so returning it would cause requests to enqueue but never process, deadlocking the REST handler.
- Runtime status API now includes `executor_ready` (boolean) to distinguish between “runtime exists but not ready” and “runtime fully operational”.
- REST handlers enforce context timeout (15s for dialogs/messages, 30s for send). ChatService methods accept `context.Context` instead of `context.Background()`.
- When `GetExecutor()` returns nil (connecting/stopped/degraded), REST calls fall through to temporary client or return cached data.
- `last_error` in runtime status is sanitized via `security.SanitizeErrorMessage()` to prevent leaking file paths, API hashes, proxy passwords, or phone numbers.
- Frontend diagnosis order: REST response → runtime status `executor_ready` + `last_error` → WebSocket state.

## executor_ready 含义

| Runtime 状态 | executor_ready | 说明 |
|-------------|----------------|------|
| `stopped` | false | 未启动 |
| `connecting` | false | Run() 未启动，executor 无法接受请求 |
| `syncing` | true | Run() 已启动，executor 可用 |
| `live` | true | Run() 已启动，executor 可用 |
| `degraded` | false | 连接异常 |
| `offline` | false | 连接断开 |

## connecting 状态不返回 executor

**原因**：`connecting` 状态时 `executor.Run()` 尚未启动。如果此时返回 executor，REST 请求会 enqueue 到 channel 但永远不会被执行（死锁）。

**行为**：`GetExecutor()` 在 `connecting` 状态返回 `nil`，REST 请求 fallback 到：
1. 缓存数据（如果有）
2. 临时客户端（如果缓存为空）

## REST fallback 规则

```
REST request → ChatService → adapter
  → GetExecutor(accountID)
    → 有 executor（live/syncing）：通过 executor.Execute()
    → 无 executor（connecting/stopped/degraded）：
      → 缓存有数据？返回缓存（source=cache, stale=true）
      → 缓存无数据？创建临时客户端（通过 AccountGate）
```

## runtime_not_ready / request_timeout

| 错误码 | 场景 | 处理 |
|--------|------|------|
| `runtime_not_ready` | executor 不可用，且无缓存 | 返回 JSON 错误，前端显示重试 |
| `request_timeout` | 超过 15s 超时 | 返回 JSON 错误，前端显示超时提示 |
| `runtime_execute_timeout` | executor 执行超时（30s） | 返回 JSON 错误 |

## 如何排查 connecting 卡住

1. 检查代理配置是否正确
2. 检查 session 文件是否有效
3. 检查 Telegram 服务是否可达
4. 查看日志中的 `runtime` 关键字，确认连接状态变化
5. 如果持续 `connecting`，检查 `last_error` 字段

## 2026-06 cache-first 行为

- Cache-first：缓存有数据时立即返回，不等待 runtime live，不等待 Telegram refresh
- `force_refresh` 参数：用户主动刷新时跳过缓存
- source 字段标识数据来源：cache/telegram/mixed
- stale 字段标识数据时效：true=缓存（可能过期）、false=实时数据
- Telegram refresh 失败不清空缓存
- 不允许无限 pending（强制超时 15s）

## Runtime 代理注入

### 修复的 Bug

在之前的版本中，`server.go` 中创建 RuntimeManagerImpl 后未调用 `SetDialer()`，导致 runtime 的 `telegram.Client` 使用直连，绕过代理配置。

修复后：
```go
// server.go
runtimeMgr := gotdadapter.NewRuntimeManager(db, key, bus, logger)
runtimeMgr.SetGate(gate)

// 注入代理 dialer
if dialer, err := BuildProxyDialerFromDB(db, key); err != nil {
    logger.Warn("Runtime dialer 初始化失败，将使用直连", "error", err)
} else if dialer != nil {
    runtimeMgr.SetDialer(dialer)
}
```

### api_proxy 对 Runtime 的影响

- `api_proxy` 类型不适用于 MTProto
- `BuildProxyDialerFromDB()` 遇到 `api_proxy` 返回明确错误
- Runtime dialer 注入失败，日志警告"将使用直连"
- Runtime 使用直连而不是 api_proxy URL
- 不会导致 infinite connecting

### Runtime 使用 MTProto

- Runtime 的 `telegram.Client` 使用 MTProto 协议
- 需要 TCP 连接到 Telegram DC 地址
- 只能通过 SOCKS5 或 HTTP CONNECT 代理
- API Proxy（HTTPS endpoint）无法承载 MTProto 连接

## Runtime update 写 ChatMessageCache

### 写入时机

当 runtime 收到 Telegram 新消息更新时：

1. `UpdateHandler.handleNewMessage()` 被调用
2. **同步写入** ChatMessageCache（`upsertMessageCache`）
3. 更新 ChatPeerCache 的 preview 和 last_message_at（`updateDialogPreview`）
4. 发布 EventBus 事件（`bus.Publish`）

**关键顺序**：Cache 写入先于 EventBus publish。这意味着任何 subscriber 看到事件时，消息已持久化在 SQLite 中。

### 写入内容

| 字段 | 来源 |
|------|------|
| AccountID | runtime account ID |
| PeerRef | mapPeerRef(msg.PeerID) |
| TelegramMessageID | msg.ID |
| Direction | "in" 或 "out" |
| SenderName | 从 users 列表查找 |
| Kind | classifyMessageKind(msg) |
| TextEncrypted | AES-256-GCM 加密 |
| SentAt | time.Unix(msg.Date, 0) |

### 去重

使用 `(account_id, peer_ref, telegram_message_id)` 复合键。如果记录已存在，更新 `text_encrypted`、`sender_name`、`kind`、`sent_at`。

## EventBus 事件与 cache 的一致性

### 事件 payload

`message.new` 事件的 Payload 是完整的 `telegramclient.Message` 结构体，包含：
- TelegramMessageID
- Text（明文，未加密）
- SentAt
- Direction
- SenderName
- Kind
- IsOutgoing
- Status

### 前端处理

1. 收到 `message.new` 后，前端写入对应 peer 的 messages query cache
2. 同时更新 dialogs cache（preview、unread、排序）
3. 非当前 peer 标记为 stale

### 如果 WebSocket 事件丢失

1. 断线重连后，前端 invalidate dialogs 和当前 peer 的 messages query
2. 切换到 stale peer 时，触发 force_refresh=true
3. 后端 force_refresh 从 Telegram 拉取最新消息，写入 ChatMessageCache
4. 即使 WebSocket 事件丢失，切换 peer 时也能通过 force refresh 补偿

## Runtime 响应网络配置变更

### 热更新机制

代理配置保存后，`handleAPISaveProxy` 调用 `RuntimeManager.OnProxySettingsChanged()`：

1. 从数据库重新读取代理配置
2. 重建 dialer（`m.dialFunc`）
3. 停止所有运行时（`StopAll()`）
4. 返回 dialer 是否可用于 MTProto

### API Proxy 下的行为

如果 proxy_type=api_proxy：
- `rebuildDialer` 返回 `available=false, err="API Proxy 不适用于 MTProto 连接"`
- `m.dialFunc` 设为 nil（直连，但不适用于 MTProto）
- 所有运行时被停止
- 前端显示 warning

### 切回可用代理后的行为

如果从 api_proxy 切回 socks5/https：
- `rebuildDialer` 返回 `available=true, err=nil`
- `m.dialFunc` 设为新 dialer
- 所有运行时被停止
- 用户可手动重新启动 runtime
- 新 runtime 使用新 dialer

### 不清空 chat cache

代理配置变更后：
- 不清空 ChatMessageCache
- 不清空 ChatPeerCache
- 不删除 session
- 不删除 account
- 只重置 runtime/network connection 状态

### REST temporary client

REST temporary client 每次请求都从 DB 读取代理配置（`newChatService()` → `BuildProxyDialerFromDB()`），因此代理变更后立即生效。

但当 runtime 处于 live/syncing 状态时，请求通过 runtime executor 走旧 client。代理变更后 runtime 被停止，请求会 fallback 到 temporary client，此时使用新配置。
