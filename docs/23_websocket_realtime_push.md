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
| `message.deleted` | 删除消息 | `{message_ids: []}` |
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
