# Media MVP

## 概述

Media MVP 为聊天界面提供媒体文件的下载和预览功能。用户可以按需下载媒体文件到本地缓存，然后在线预览或在新窗口中打开。

## 支持的媒体类型

| 类型 | 预览方式 | 说明 |
|------|---------|------|
| photo | 内嵌 `<img>` | 下载后在消息气泡内显示缩略图 |
| video | 内嵌 `<video>` | 下载后在消息气泡内播放 |
| voice | 内嵌 `<audio>` | 下载后在消息气泡内播放 |
| audio | 内嵌 `<audio>` | 下载后在消息气泡内播放 |
| document | 新窗口打开 | 下载后提供"打开"按钮 |
| sticker | 内嵌 `<img>` | 下载后显示贴纸图片 |
| animation | 新窗口打开 | 下载后提供"打开"按钮 |

## 媒体状态机

```
none → downloading → cached → (预览/打开)
                  ↘ failed
```

- **none**: 未下载，显示下载按钮
- **downloading**: 正在下载，显示加载动画
- **cached**: 已缓存，显示预览或打开按钮
- **failed**: 下载失败，显示错误信息

## API 端点

### GET /api/media/{messageId}/status

获取媒体文件的缓存状态。

**查询参数:**
- `peer_ref`: 对话标识
- `account_id`: 账号 ID

**响应:**
```json
{
  "ok": true,
  "status": "cached",
  "file_name": "photo.jpg",
  "mime_type": "image/jpeg",
  "file_size": 102400,
  "available": true
}
```

### POST /api/media/{messageId}/download

触发媒体文件下载到本地缓存。

**查询参数:**
- `peer_ref`: 对话标识

**响应:**
```json
{
  "ok": true,
  "status": "cached",
  "file_name": "photo.jpg",
  "mime_type": "image/jpeg",
  "file_size": 102400
}
```

### GET /api/media/{messageId}/content

获取已缓存媒体文件的二进制内容。

**查询参数:**
- `peer_ref`: 对话标识

**响应:** 媒体文件的原始二进制数据，Content-Type 根据文件类型设置。

## 前端实现

### 文件结构

```
frontend/src/
├── api/
│   └── media.ts          # 媒体 API 客户端
├── features/chat/
│   └── MediaMessage.vue  # 媒体消息组件（已重写）
└── types/
    └── chat.ts           # ChatMessage 类型（已扩展）
```

### 组件状态管理

`MediaMessage.vue` 使用组件内部状态管理媒体下载流程：

- `mediaStatus`: 响应式状态，跟踪当前下载状态
- `contentUrl`: 缓存命中后的访问 URL
- `mediaError`: 下载失败时的错误信息

### 安全边界

- 媒体下载通过已认证的 Web Session（CSRF token + cookie）
- 媒体文件存储在服务器本地缓存目录，不暴露原始路径
- 不支持批量下载，每次只能下载单个文件
- 媒体文件不存储在数据库中，仅缓存在文件系统
- 路径穿越防护：`sanitizeLocalPath` 和 `sanitizeFileName` 清理文件路径和名称
- 文件大小限制：最大 100MB（`MaxFileSize`）
- 下载锁定：`sync.Mutex` 防止并发下载同一文件
- Stale 恢复：启动时将卡在 downloading 状态超过 5 分钟的记录重置为 failed
- Content-Security-Policy: `default-src 'none'; style-src 'unsafe-inline'`
- X-Content-Type-Options: `nosniff`
- Content-Disposition 使用 sanitize 后的文件名

## i18n

媒体相关文案走 i18n（media.photo, media.document, media.sticker, media.video, media.voice, media.audio, media.download, media.downloading, media.downloadFailed, media.unsupported 等），10 种语言支持。
