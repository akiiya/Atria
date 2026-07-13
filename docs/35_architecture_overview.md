# 架构总览

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Browser (Vue 3 SPA)                     │
│  ┌─────────┐ ┌────────┐ ┌────────┐ ┌──────┐ ┌───────────┐ │
│  │  Chat   │ │Contacts│ │ Search │ │Audit │ │ Dashboard │ │
│  └────┬────┘ └───┬────┘ └───┬────┘ └──┬───┘ └─────┬─────┘ │
│       └──────────┴──────────┴─────────┴───────────┘         │
│                          │ HTTP/WS                           │
└──────────────────────────┼───────────────────────────────────┘
                           │
┌──────────────────────────┼───────────────────────────────────┐
│                    Gin HTTP Server                            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐ │
│  │  Auth    │ │  CSRF    │ │  Router  │ │  WebSocket Hub │ │
│  └──────────┘ └──────────┘ └────┬─────┘ └───────┬────────┘ │
│                                 │               │            │
│  ┌──────────────────────────────┼───────────────┘            │
│  │         Server Handlers      │                            │
│  │  ┌────────┐ ┌────────┐ ┌────┴───┐ ┌──────────────────┐  │
│  │  │  Chat  │ │ Search │ │ Media  │ │ Accounts/Settings│  │
│  │  └───┬────┘ └───┬────┘ └───┬────┘ └────────┬─────────┘  │
│  └──────┼──────────┼──────────┼───────────────┘             │
│         │          │          │                              │
│  ┌──────┴──────────┴──────────┴─────────────────────────┐   │
│  │              internal/chat (ChatService)              │   │
│  │         中立 DTO，不依赖 gotd/td                       │   │
│  └──────────────────────┬───────────────────────────────┘   │
│                         │                                    │
│  ┌──────────────────────┴───────────────────────────────┐   │
│  │        telegramclient.ClientAdapter (接口)             │   │
│  │   ┌─────────────────┐    ┌──────────────────────┐    │   │
│  │   │  gotd/adapter   │    │  tdlib/adapter (预留) │    │   │
│  │   └────────┬────────┘    └──────────────────────┘    │   │
│  └────────────┼─────────────────────────────────────────┘   │
│               │                                              │
│  ┌────────────┴─────────────────────────────────────────┐   │
│  │              GORM + SQLite/PostgreSQL                  │   │
│  │  ┌──────────┐ ┌────────────┐ ┌────────────────────┐  │   │
│  │  │ Accounts │ │ Peer Cache │ │ Message Cache      │  │   │
│  │  │ Sessions │ │ (encrypted)│ │ (AES-256-GCM)      │  │   │
│  │  └──────────┘ └────────────┘ └────────────────────┘  │   │
│  └───────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
               │
┌──────────────┴──────────────────────────────────────────────┐
│                   Telegram MTProto                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  gotd/td → updates.Manager → UpdateHandler → EventBus │   │
│  └──────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

## 数据流

### 消息读取

```
Browser → GET /api/chats/:peer_ref/messages
  → ChatService.GetMessages()
    → 检查 ChatMessageCache (SQLite)
    → 命中：解密 TextEncrypted (AES-256-GCM) → 返回
    → 未命中：ClientAdapter.GetRecentMessages() → Telegram API
      → 写入 ChatMessageCache → 返回
```

### 实时推送

```
Telegram → MTProto updates.Manager
  → UpdateHandler.Handle()
    → mapUpdateNewMessage() → 中立 DTO
    → upsertMessageCache() → SQLite
    → updateDialogPreview() → unread_count++
    → EventBus.Publish()
      → wsUpdateSink.Send()
        → WebSocket → Browser
          → handleRealtimeEvent() → TanStack Query cache 更新
```

### Mark Read

```
Browser → POST /api/chats/:peer_ref/read
  → ChatService.MarkRead()
    → ClientAdapter.MarkRead()
      → gotd: messages.readHistory / channels.readHistory
    → 更新 ChatPeerCache.unread_count = 0
    → 返回成功
  → Browser: patchDialogUnreadCount() → 角标消失
```

## 安全边界

```
┌─────────────────────────────────────────────┐
│              internal/chat                    │
│  ✅ 只依赖 telegramclient (中立接口)          │
│  ❌ 不得 import gotd/td                      │
└─────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────┐
│        telegramclient (中立 DTO)              │
│  ✅ Dialog, Message, Contact, PeerInfo       │
│  ✅ ClientAdapter 接口                       │
│  ❌ 不含 gotd 类型                           │
└─────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────┐
│     telegramclient/gotd (适配器实现)          │
│  ✅ 所有 gotd 类型隔离在此包内                │
│  ✅ tg.Dialog → telegramclient.Dialog        │
│  ✅ tg.Message → telegramclient.Message      │
└─────────────────────────────────────────────┘
```

## 加密层级

| 数据 | 加密方式 | 存储位置 |
|------|---------|---------|
| 管理员密码 | bcrypt | admins 表 |
| API Hash | AES-256-GCM + AAD | api_credentials 表 |
| 手机号 | AES-256-GCM + AAD | telegram_accounts 表 |
| 消息正文 | AES-256-GCM + AAD | chat_message_cache 表 |
| Session 文件 | AES-256-GCM | data/sessions/ |
| access_hash | AES-256-GCM + AAD | chat_peer_cache 表 |
| Web Session | AES-256-GCM + AAD | Cookie (HttpOnly) |

## 中间件链

```
请求 → TLS检测 → Recovery → 路由匹配
  → Auth中间件（需认证路由）
  → CSRF中间件（POST路由）
  → Handler
```
