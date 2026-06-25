# 31 - 搜索 MVP

## 概述

搜索 MVP 提供本地消息缓存的全文搜索功能。搜索在已缓存的消息中进行，通过解密后匹配关键词，不调用 Telegram 搜索 API。

## 设计决策

- **仅搜索本地缓存：** 不调用 Telegram search API，避免网络请求和速率限制
- **解密后匹配：** 消息正文在数据库中加密存储（AES-256-GCM），搜索时先解密再匹配
- **当前账号隔离：** 搜索范围限定为当前选中的 Telegram 账号
- **可选 peer 过滤：** 支持按 peer_ref 缩小搜索范围到特定聊天

## API

### GET /api/search/messages

搜索本地消息缓存。

**查询参数：**
- `q` (必需) - 搜索关键词
- `peer_ref` (可选) - 限定搜索范围到特定会话
- `limit` (可选, 默认 20, 最大 100) - 返回结果数
- `offset` (可选, 默认 0) - 分页偏移

**响应：**
```json
{
  "ok": true,
  "results": [
    {
      "peer_ref": "u_123456",
      "message_id": 789,
      "sender_name": "John",
      "text_snippet": "...matching text around the query...",
      "sent_at": "2026-06-24 10:30:00",
      "is_outgoing": false
    }
  ],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

## 前端

### 路由

- `/search` - 搜索页面

### 组件

- `SearchView.vue` - 搜索页面，包含搜索输入框、结果列表、分页

### API

- `frontend/src/api/search.ts` - 搜索 API 封装

## 安全

- 搜索查询不记录到审计日志
- 搜索范围限定为当前账号
- 解密操作在内存中进行，不写入日志

## 限制

- 仅搜索已缓存的消息（最多 500 条/peer）
- 不支持搜索图片、文件等非文本消息
- 搜索性能取决于缓存大小
