# WebSocket 实时推送

## 为什么引入 WebSocket

Atria 之前所有聊天数据通过 REST API 加载，用户需要手动刷新才能看到新消息。WebSocket 让后端可以将 Runtime EventBus 中的实时事件推送到前端，实现近实时的聊天体验。

## 设计原则

- **WebSocket 只消费 EventBus**：不直接接触 gotd，不直接请求 Telegram
- **中立事件类型**：使用 `telegramclient.UpdateEvent`，不包含 gotd 类型
- **TDLib 可替换**：未来 TDLib runtime 也可以 publish 同样的 UpdateEvent，WebSocket 层不需要修改

## Endpoint

```
GET /api/realtime/ws
```

## 连接鉴权

1. 复用 cookie session（`atria_session`）
2. 未登录返回 401
3. Origin 校验（同源）
4. 只订阅当前 selected account 的 EventBus
5. 不允许客户端传任意 account_id

## 事件 Envelope

```json
{
  "type": "message.new",
  "event_id": "evt_xxx",
  "account_id": 1,
  "peer_ref": "u_xxx",
  "created_at": "2026-xx-xxTxx:xx:xxZ",
  "payload": {}
}
```

## 支持的事件类型

| 事件 | 说明 | Payload |
|------|------|---------|
| `message.new` | 新消息 | ChatMessage DTO |
| `message.edited` | 编辑消息 | ChatMessage DTO |
| `message.deleted` | 删除消息 | `{telegram_message_ids: []}` |
| `dialog.upserted` | 会话更新 | Dialog DTO |
| `sync.started` | 同步开始 | state |
| `sync.done` | 同步完成 | state |
| `sync.failed` | 同步失败 | state + error |
| `account.connected` | 连接成功 | state |
| `account.disconnected` | 断开连接 | state |

## 安全策略

- 不返回 access_hash、api_hash、proxy_password、session_path
- 不记录 message body（只记录 text_len）
- Payload 使用中立 DTO，不包含 gotd 原始类型

## 前端实现

### WebSocket Client (`frontend/src/realtime/ws.ts`)

- 基于浏览器原生 WebSocket
- 自动重连（指数退避，1s → 30s max）
- 连接状态：connecting / connected / reconnecting / disconnected / error
- 页面卸载时自动关闭

### Query Cache Patch (`frontend/src/realtime/handler.ts`)

收到事件后局部 patch TanStack Query cache，不整页刷新：

- `message.new`：插入 messages cache + 更新 dialogs preview
- `message.edited`：替换 messages cache 中的文本
- `message.deleted`：从 messages cache 中移除
- `dialog.upserted`：更新 dialogs cache

### 滚动行为

- 当前 peer 收到新消息 + nearBottom → 自动滚到底部
- 当前 peer 收到新消息 + 不在底部 → 显示"有新消息"提示
- 非当前 peer 收到消息 → 只更新 DialogList，不影响当前滚动
- edited/deleted → 不强制滚动

### 断线重连补偿

1. 断线后状态变为 reconnecting
2. 不清空现有消息
3. 重连成功后 invalidate dialogs + current messages query
4. 不做全量刷新

### 发送消息去重

- REST SendText 返回后替换 optimistic message
- WebSocket 可能收到同一条 message.new
- 按 `id`（telegram_message_id）去重，不重复显示

## 与 Runtime 的关系

- WebSocket 不负责创建 Telegram client
- 前端先调用 runtime status/start API
- 再连接 WebSocket
- Runtime 未启动时 WebSocket 仍可连接，但只收到状态事件

## Dev/Test 事件注入

### Endpoint

```
POST /api/realtime/dev/publish
```

### 默认关闭

仅当环境变量 `ATRIA_DEV_REALTIME_TEST=1` 时可用。默认返回 404。

### 用途

1. 后端自动化测试 WebSocket 推送
2. 前端本地手动验证 Query patch
3. 不依赖真实 Telegram 的端到端 UI 验收

### 请求格式

```json
{
  "type": "message.new",
  "peer_ref": "u_123",
  "payload": { "text": "test message" }
}
```

### 白名单事件

- `message.new`
- `message.edited`
- `message.deleted`
- `dialog.upserted`
- `sync.started`
- `sync.done`
- `sync.failed`

### 安全约束

- 必须鉴权（cookie session）
- 只能发布到当前 selected account
- 不允许传任意 account_id
- 事件 payload 走同样的 sanitize 路径
- 不写 Telegram
- 不访问真实 Telegram
- 生产环境默认关闭

## 真实 Telegram 手动验收步骤

1. 启动 `bin/atria.exe serve`
2. 登录 Atria
3. 选择已登录 Telegram 账号
4. 打开 `/app/#/chats`
5. 确认 runtime status 为 `live`
6. 打开浏览器 DevTools Network，确认 `/api/realtime/ws` 已连接
7. 打开一个会话并停留在底部
8. 用手机 Telegram 或官方客户端给该账号发送一条消息
9. 不刷新页面
10. 确认新消息自动显示
11. 上滑到历史位置
12. 再发送一条消息
13. 确认页面不被强制拉到底部，并显示"有新消息"
14. 点击"有新消息"，确认滚到底部
15. 切到另一个会话
16. 给非当前会话发消息
17. 确认当前消息区不变，左侧会话 preview/unread 更新
18. 断开网络或停止服务，确认 WebSocket reconnecting
19. 恢复后确认 reconnect 并补偿 invalidate
20. 检查日志不含敏感字段和 message body

## WebSocket 链路排查顺序

如果 WebSocket 未收到事件，按以下顺序排查：

1. **Runtime 状态**：`GET /api/chats/runtime/status` 确认 `state=live`
2. **EventBus**：检查 runtime 日志是否有 `新消息处理完成` 或 `消息编辑处理完成`
3. **WebSocket 连接**：DevTools Network 确认 WS 连接建立
4. **事件接收**：DevTools WS 面板确认收到 JSON 事件
5. **Query patch**：DevTools Console 检查 TanStack Query cache 是否更新
6. **UI 更新**：确认 Vue 组件是否重新渲染

## 后续扩展

- 媒体消息实时更新
- read state 同步
- typing indicator
- reaction 更新

## 2026-06 realtime hardening addendum

- Dev publish remains disabled by default and is available only when `ATRIA_DEV_REALTIME_TEST=1`.
- `POST /api/realtime/dev/publish` is protected by the same auth cookie and CSRF cookie/header mechanism as other POST APIs.
- Dev publish always resolves the selected account server-side; client supplied `account_id` is ignored and cannot override the target account.
- Dev publish never calls Telegram and only publishes sanitized neutral `UpdateEvent` payloads into EventBus.
- `message.deleted` uses `payload.telegram_message_ids` as the canonical field. `message_ids` is only a legacy input fallback and is never serialized as the primary field.
- `ChatMessage.telegram_message_id` is the REST/WebSocket deletion and deduplication key. `ChatMessage.id` mirrors it for REST compatibility and must not be treated as a cross-layer deletion contract.
- Optimistic outgoing messages use `local_id` / `client_pending_id`, `pending=true`, and `status="sending"` before send. REST success replaces the local message, and later WebSocket `message.new` deduplicates by `telegram_message_id` or local id.
- Query patch supports flat `messages`, `older_messages`, and paged `pages[].messages` without clearing existing history. `sync.failed` does not clear messages, and reconnect only invalidates scoped queries.
- Manual dev publish acceptance must cover disabled-by-default, unauthenticated, missing/invalid CSRF, selected-account-only, `message.new`, `message.edited`, `message.deleted` by `telegram_message_ids`, dialog preview updates, and optimistic outgoing deduplication.
- Real Telegram acceptance remains manual only: automated tests must not connect to Telegram.

## 2026-06 chat loading deadlock fix

- WebSocket failure or runtime `connecting` state must not block REST dialogs/messages loading. REST queries are independent of WebSocket connection state.
- `GetExecutor()` returns executor only when runtime state is `live` or `syncing`; `connecting` returns nil so REST falls back to temporary client or cache.
- REST handlers (`/api/chats/dialogs`, `/api/chats/:peer_ref/messages`) enforce 15-second context timeout; send text uses 30 seconds. `context.Background()` replaced with request context.
- ChatService methods (`ListDialogs`, `GetMessages`, `LoadOlderMessages`, `SendText`) now accept `context.Context` for timeout propagation.
- Runtime status API (`GET /api/chats/runtime/status`) now returns `executor_ready` (boolean, true only in `live`/`syncing`) and sanitized `last_error` (file paths, hex strings, phone numbers redacted).
- Frontend `apiGet`/`apiPost` use `AbortController` with 30-second timeout to prevent infinite fetch.
- Frontend dialogs skeleton shows "加载时间较长" hint after 10 seconds with a retry button.
- Runtime badge tooltip shows `last_error` and executor status for diagnostics.
- Diagnosis order for loading issues: REST dialogs/messages response → runtime status `executor_ready` + `last_error` → WebSocket connection state.

## WebSocket 失败不阻塞 REST

- WebSocket 连接状态不影响 REST API 调用
- REST dialogs/messages 使用独立的 HTTP 请求，与 WebSocket 无关
- WebSocket reconnect 时只 invalidate 对应的 query，不影响首屏加载
- `refetchOnWindowFocus: false` 避免窗口聚焦时触发不必要的请求

## 非当前 peer message.new 处理策略

### 问题

收到非当前会话的新消息后，如果只更新 dialogs cache，用户切换到该会话时可能看不到最新消息（因为 TanStack Query staleTime=30s 内不会 refetch）。

### 解决方案：双保险

**保险 1：WebSocket message.new 直接写 messages cache**

收到 `message.new` 时，无论是否为当前 peer，都写入对应 peer 的 messages query cache：
```
upsertMessageInMessagesCache(queryClient, accountId, peerRef, msg)
```
这样切换到该 peer 时，TanStack Query cache 已有最新消息。

**保险 2：peer stale 标记 + 切换时 reconcile**

- 收到非当前 peer 的 `message.new` 时，标记该 peer 为 stale
- 收到 `dialog.upserted` 时，也标记该 peer 为 stale
- 用户切换到 stale peer 时，`MessagePanel` 检测到 stale 标记，触发 `force_refresh=true` 的 REST 请求
- 后端 `force_refresh=true` 跳过缓存，直接从 Telegram 拉取最新 50 条消息
- 拉取成功后清除 stale 标记

### dialogs query 与 messages query 的一致性

- dialogs cache 通过 WebSocket 实时更新（preview、unread、排序）
- messages cache 通过 WebSocket 实时更新（当前 peer + 非当前 peer）
- 切换 peer 时，如果 peer 是 stale，force refresh 确保 messages 与 Telegram 同步
- 后端 ChatMessageCache 由 runtime update handler 同步写入，先于 EventBus publish

### peer stale 标记

存储在前端 Pinia store (`useChatStore`) 的 `stalePeers: Set<string>` 中。

触发 stale 的条件：
1. `message.new` 到达非当前 peer
2. `dialog.upserted` 到达任何 peer

清除 stale 的条件：
1. 用户切换到该 peer 并完成 force refresh
2. 切换账号时清空所有 stale

### force_refresh/latest refresh 策略

- `force_refresh=true` 只拉最近 50 条，不全量历史
- 后端写入 ChatMessageCache 后返回
- 如果 Telegram 不可达，返回 cache + stale/error
- 错误不能让前端清空已有消息

### 不全量刷新

- 只对 stale peer 触发 force refresh
- 非 stale peer 使用 cache-first（staleTime=30s 内不 refetch）
- 不扫所有 peer 的 messages cache
- 不破坏 older pagination

## Reconnect 只补偿，不影响首屏

- WebSocket reconnect 成功后，invalidate 丢失事件期间的查询
- 补偿 invalidate 范围：`['dialogs', accountId]` + `['messages', accountId, peerRef]`
- 不影响首屏 cache-first 加载
- 不清空已有缓存数据

## 排查顺序：dialogs/messages → runtime status → websocket

1. **dialogs/messages**：检查 REST 响应，确认 source 和 stale
2. **runtime status**：检查 `executor_ready` 和 `last_error`
3. **websocket**：检查连接状态和事件接收

## 2026-06 cache-first + force_refresh

- Cache-first：缓存有数据时立即返回，不等待 WebSocket 或 runtime
- `force_refresh=true`：用户主动刷新时跳过缓存
- source 字段标识数据来源：cache/telegram/mixed
- stale 字段标识数据时效：true=缓存（可能过期）、false=实时数据
- Telegram refresh 失败不清空缓存

## 消息区排序一致性

- `message.new` 合并后必须保持 `sent_at ASC` 排序。
- `message.deleted` / `message.edited` 不得破坏排序（只 filter/map，不重新排序）。
- 新 dialog 插入只影响 DialogList DESC 排序，不影响 MessagePanel ASC。
- peer switch reconcile merge 后仍保持 ASC。
- 统一使用 `sortMessagesAsc()` 排序函数。
- `sent_at` 相同时按 `telegram_message_id ASC` 兜底。
- 预览文本截断使用 `safeTruncateText()`，不破坏 emoji。

## 会话唯一性

- `dialog.upserted` 必须使用 canonical `peer_ref`（与 ListDialogs 一致）。
- `message.new` 的 message payload 必须携带正确的 `PeerRef`（由 `mapMessage` 设置）。
- 前端 `handleDialogUpserted` 忽略 `peer_ref` 为空的 dialog。
- 前端 `ChatView` 对 dialogs 按 `peer_ref` 防御性去重。
- `message.new` 缺少 dialog 时构造的 minimal dialog 使用 canonical `peer_ref`，后续 REST dialog 可正确合并。
- 不允许 unknown peer 生成错误临时 peer_ref 导致重复会话。
