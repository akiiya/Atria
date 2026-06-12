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

## 后续扩展

- 媒体消息实时更新
- read state 同步
- typing indicator
- reaction 更新
