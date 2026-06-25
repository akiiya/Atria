# 34 - 群组/频道 MVP

## 概述

增强 Peer 类型识别，支持群组（Group）、超级群组（Supergroup）和频道（Channel）的区分显示，
并在前端展示成员数、类型标签等信息。

## 支持的 Peer 类型

| PeerType    | 说明                      | peer_ref 前缀 | 需要 access_hash |
|-------------|--------------------------|---------------|-----------------|
| `user`      | 普通用户                   | `u_`          | 是              |
| `bot`       | 机器人（user.Bot=true）    | `u_`          | 是              |
| `chat`      | 基础群组（最多 200 人）     | `c_`          | 否              |
| `supergroup`| 超级群组（Megagroup=true）  | `ch_`         | 是              |
| `channel`   | 频道（Broadcast=true）     | `ch_`         | 是              |

## 显示字段

### Dialog 列表项
- **类型图标**：bot 显示 🤖，chat/supergroup 显示 👥，channel 显示 📢
- **类型标签**：在预览行显示翻译后的类型名称（user 不显示）
- **成员数**：`member_count` 字段（从 Telegram API 获取）
- **标志位**：`flags` 字段，逗号分隔：`verified`, `scam`, `fake`, `restricted`, `broadcast`, `megagroup`

### 消息头部
- 在标题旁显示类型标签（如 "超级群组"、"频道"）

## 数据流

```
gotd (tg.Dialog) 
  → mapDialog() 提取 PeerType/MemberCount/Flags
  → telegramclient.Dialog (neutral DTO)
  → mapNeutralDialogToChatDialog() 
  → chat.Dialog (service DTO)
  → 前端 Dialog
```

## 缓存策略

### ChatPeerCache 新增字段
- `member_count`：成员数
- `flags`：频道标志位（verified/scam/fake/restricted/broadcast/megagroup）
- `description`：描述（预留）

### 缓存写入
- `upsertPeerCacheFromDialog` 在 ListDialogs 时写入
- 同时更新 `member_count` 和 `flags` 字段

### 缓存读取
- `listDialogsFromCache` 从缓存读取时包含 `member_count` 和 `flags`

## API 端点

### GET /api/chats/peers/:peer_ref/info

返回 peer 详细信息（从缓存读取）。

**响应：**
```json
{
  "ok": true,
  "peer_ref": "ch_123456",
  "peer_type": "supergroup",
  "title": "群组名称",
  "username": "group_name",
  "member_count": 1234,
  "flags": "verified,megagroup",
  "description": "",
  "source": "cache"
}
```

**错误响应：**
```json
{
  "ok": false,
  "message": "会话信息不存在"
}
```

## 安全边界

- `access_hash` 仍使用 AES-256-GCM 加密存储
- `flags` 和 `member_count` 为非敏感元数据，明文存储
- API 端点需要认证（authMiddleware）
- 不暴露原始 Session 数据
- 不记录敏感字段到日志

## 数据库迁移

Migration 13：为 `chat_peer_cache` 添加字段
- `member_count` (INTEGER, default 0)
- `flags` (VARCHAR(128))
- `description` (VARCHAR(1024))

## 前端国际化

新增 i18n 键：
- `peerType.bot`：Bot / 机器人
- `peerType.group`：Group / 群组
- `peerType.supergroup`：Supergroup / 超级群组
- `peerType.channel`：Channel / 频道

注意：user 类型不显示标签（默认类型），无需 i18n 键。

支持 10 种语言：en, zh-CN, zh-TW, ja, ko, de, fr, es, pt-BR, ru。
