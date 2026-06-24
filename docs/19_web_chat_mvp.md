# Web 聊天 MVP

> 聊天底层协议实现已通过 `telegramclient.ClientAdapter` 隔离，当前实现是 gotd，未来可替换 TDLib。详见 `docs/21_telegram_client_adapter.md`。
>
> 聊天实时推送已通过 WebSocket 接入（`docs/23_websocket_realtime_push.md`）。支持 message.new/edit/delete、dialog upsert、sync status 事件。
>
> 后端 runtime 已可启动和观测。前端聊天页显示 runtime 状态指示器（connecting/syncing/live/degraded/offline/stopped）。
>
> 当前实时推送是基础版。dev publish 可用于本地验收但生产默认关闭。默认入口已 canonicalize 到 `/app/#/dashboard`。旧 Go Template 不再作为登录后默认 dashboard。真实 Telegram 手动验收仍是进入 main 前必做项。

## 支持范围

- 会话列表：查看当前 Telegram 账号的最近会话
- 消息历史：查看某个会话的最近消息
- 单会话文本发送：向单个已存在会话发送一条文本消息

## 明确不支持

- 群发 / 批量发送
- 自动回复
- 定时消息
- 联系人采集
- 群成员采集
- 风控规避

## peer_ref 设计

peer_ref 是服务端生成的不透明引用，不暴露 access_hash 明文。

- 格式：`u_ID`、`c_ID`、`ch_ID`（分别对应 user、chat、channel）
- access_hash 通过 AES-256-GCM 加密后存储在 `chat_peer_cache` 表
- peer_ref 绑定 account_id，不允许跨账号使用
- 前端只能看到 peer_ref，无法获取 access_hash

## 旧账号兼容

聊天模块必须兼容在聊天功能上线前已经接入的账号。

- `chat_peer_cache` 为空不代表没有账号，只代表还没有拉取过会话列表
- 当前账号解析使用 `resolveCurrentAccountID`，与 topbar 保持一致
- 如果 `selected_account_id` cookie 无效，自动 fallback 到第一个有效账号
- 旧账号如果缺少 `account_sessions` 记录，启动时迁移 4 会自动补齐
- 不要求用户重新登录旧账号，除非 session 本身已经失效
- 迁移 4 不访问 Telegram 网络，不修改 session 文件，不删除旧账号

## 代理要求

聊天相关 MTProto 调用（ListDialogs、GetMessages、SendText）必须使用系统 API 网络代理配置。

代理配置来源：`system_settings` 表中的 `proxy_*` 字段。

proxy_password 缺失时视为空密码，不打印 record not found 噪音日志。
proxy_password 解密失败时返回 proxy_config_invalid，不静默直连。

### 代理配置热生效

代理配置保存后，运行时立即生效，不需要重启服务。

- 保存 SOCKS5/HTTPS 后，runtime 停止旧连接，下次 start 使用新代理
- 切换代理后无需重启服务
- 已有聊天缓存仍可查看，不清空

### Legacy API Proxy 处理

如果旧数据库中已保存 `proxy_type=api_proxy`（API Proxy 已移除）：
- 聊天页显示明确错误，不无限 skeleton
- Runtime status 显示 `proxy_config_invalid`
- 已有缓存仍可查看
- 用户需要在设置中重新选择 SOCKS5 / HTTPS CONNECT / none

## 安全日志

- 不记录完整消息正文（只记录 text_len）
- 不记录 access_hash 明文
- 不记录 api_hash
- 不记录 proxy_password
- 不记录 session path

## 缓存策略

### 两层缓存架构

1. **后端 SQLite 缓存**：持久化保存最近会话和消息，重启后仍能秒开
2. **前端 TanStack Query 缓存**：页面内切换会话更流畅，避免重复请求

### ChatPeerCache（会话缓存）

- 存储 peer 信息（peer_ref、peer_type、peer_id、加密 access_hash、title、username）
- 存储会话元信息（last_message_preview、last_message_at、unread_count、is_pinned、is_muted）
- 按 account_id 隔离，不返回其它账号缓存
- access_hash 使用 AES-256-GCM 加密，不返回前端明文

### ChatMessageCache（消息缓存）

- 按 `account_id + peer_ref + telegram_message_id` 唯一索引
- 消息正文（text、caption）使用 AES-256-GCM 加密存储
- 每个 peer 最多缓存 500 条最近消息
- 不做全量历史扫描
- 不做自动后台同步
- 不做浏览器长期正文缓存（IndexedDB/localStorage）

### 消息历史分段加载策略（Latest-Window 模式）

打开会话时只渲染最近一页消息，不渲染完整历史。用户上滑时才分页加载更早消息。

**常量定义：**
- `INITIAL_LATEST_LIMIT = 20`：首屏加载消息数
- `LOAD_OLDER_LIMIT = 30`：每次上滑加载历史消息数

首屏加载（latest page）：
1. 打开会话，请求 `GET /api/chats/:peer_ref/messages?limit=20`
2. 后端返回最新 20 条（ORDER BY telegram_message_id DESC LIMIT 20，再反转为 ASC）
3. 前端只渲染这 20 条作为 visible window
4. 切换会话时清空 olderPages，只保留 latest page
5. 默认定位到最新消息底部
6. 消息不足一屏时，通过 CSS `margin-top: auto` 靠底部显示，不出现突兀滚动条
7. 消息超过一屏时，通过 `scheduleScrollToBottom` 定位到底部

向上加载更早消息（older pagination）：
1. 用户上滑接近顶部时触发
2. 请求 `GET /api/chats/:peer_ref/messages?before_id=xxx&limit=30`
3. 后端返回 before_id 之前的消息
4. 前端 prepend 到 olderPages
5. 滚动锚点保持，视图不跳动（MessageList 内部管理 anchor）
6. 没有更多历史时停止请求

Visible Window 结构：
- `recentMessages`：latest page（来自 API / TanStack Query cache）
- `olderPages`：用户上滑加载的历史页（独立管理，peer switch 时清空）
- `allMessages` = olderPages + recentMessages，按 sent_at ASC，去重

CSS 底部锚定：
- `.message-scroll-container` 使用 `display: flex; flex-direction: column`
- `.message-list-anchor` 使用 `margin-top: auto`，消息不足一屏时靠底部显示
- 消息超过一屏时 `margin-top: auto` 自动变为 0，不影响正常滚动

边界状态：
- `has_older=true` 表示还可能有更早历史
- `has_older=false` 表示已经到顶部
- `loading_older` 表示正在加载更早历史
- `older_error` 表示加载失败

### before_id / offset_id 映射

- 前端传递 `before_id` = 当前已加载最早的 `telegram_message_id`
- 后端 adapter 将其映射为 gotd `MessagesGetHistoryRequest.OffsetID`
- Telegram 返回 `OffsetID` 之前的消息

### Cache-first 加载流程

1. 用户进入聊天页，前端请求 `/api/chats/dialogs`
2. 后端先读 `chat_peer_cache` 表，立即返回缓存（source=cache, stale=true）
3. 后台异步刷新 Telegram，成功后更新缓存
4. Telegram 刷新失败时保留缓存数据，返回 stale=true
5. 前端 TanStack Query staleTime=30s，避免重复请求

### 非当前会话新消息处理

收到非当前会话的新消息后，必须保证进入该会话时能看到最新消息。

**双保险策略：**

1. **WebSocket message.new 直接写 messages cache**：无论是否为当前 peer，都写入对应 peer 的 messages query cache
2. **peer stale 标记 + 切换时 reconcile**：标记非当前 peer 为 stale，切换时触发 force_refresh

**切换 peer 时的行为：**

1. 先显示已有 cache（如果有）
2. 如果 peer 是 stale，立即调用 `force_refresh=true` 拉取最新消息
3. merge 后按 telegram_message_id 去重
4. 不清空旧消息，不覆盖 older pages

**dialog preview 与 message cache 的一致性：**

- dialogs cache 通过 WebSocket 实时更新（preview、unread、排序）
- messages cache 通过 WebSocket 实时更新（写入非当前 peer）
- 后端 ChatMessageCache 由 runtime update handler 同步写入
- 切换 peer 时 force refresh 确保 messages 与 Telegram 同步

### 消息缓存加密

- 存储时：`crypto.EncryptString(key, text, "atria:msg:v1")`
- 读取时：`crypto.DecryptString(key, encrypted, "atria:msg:v1")`
- 密钥来自 secret.key，与 api_hash 加密共用同一密钥
- 不在日志中记录完整消息正文

## 路由

| 路由 | 方法 | 说明 |
|------|------|------|
| `/chats` | GET | 会话列表页面 |
| `/chats/:peer_ref` | GET | 消息历史页面 |
| `/api/chats/:peer_ref/messages` | GET | 获取消息历史（支持分页） |
| `/api/chats/:peer_ref/messages` | POST | 发送消息（JSON） |

### GET /api/chats/:peer_ref/messages 参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 20 | 消息数量，最大 100（首屏 20，older pagination 30） |
| `before_id` | int | 0 | 加载此 ID 之前的消息，0 表示最近消息 |
| `prefer_cache` | bool | true | 优先从缓存读取 |

### 响应结构

```json
{
  "ok": true,
  "messages": [],
  "source": "cache|telegram|mixed",
  "stale": false,
  "has_older": true,
  "oldest_message_id": 123,
  "newest_message_id": 456
}
```

## 错误码

| code | 说明 |
|------|------|
| `no_current_account` | 请先接入 Telegram 账号 |
| `session_invalid` | 当前账号 Session 已失效，请重新接入 |
| `peer_invalid` | 会话不存在或已过期 |
| `peer_incomplete` | 会话信息不完整 |
| `text_empty` | 消息内容不能为空 |
| `text_too_long` | 消息内容超过 4096 字符 |
| `bulk_not_supported` | 当前版本仅支持单会话发送 |
| `proxy_connect_failed` | 无法连接代理，请检查 API 网络代理配置 |
| `proxy_auth_failed` | 代理认证失败，请检查代理用户名和密码 |
| `telegram_timeout` | 连接 Telegram 超时，请稍后重试或检查代理 |
| `telegram_error` | Telegram 返回异常，请稍后重试或检查日志 |
| `api_key_invalid` | Telegram API Key 不可用 |
| `flood_wait` | Telegram 限制请求过快，请稍后再试 |
| `auth_restart` | Telegram 要求重新开始认证，请重新接入账号 |
| `account_deactivated` | 该 Telegram 账号不可用或已被停用 |
| `network_error` | 网络异常，请检查网络连接或代理配置 |

## 真实聊天错误诊断

/chats 真实调用 Telegram MessagesGetDialogs 失败时，错误分类流程：

1. 检查 context 错误（DeadlineExceeded → telegram_timeout）
2. 检查代理错误（net.OpError → proxy_connect_failed）
3. 使用 tgerr.As 提取 Telegram RPC 错误（AUTH_KEY_UNREGISTERED → session_invalid）
4. 检查 net.Error（timeout → telegram_timeout）
5. 检查 mtproto.MTProtoError 类型
6. 未知错误归类为 telegram_error（不是 network_error）

proxy_password 缺失是正常情况（代理无密码），不会触发 record not found 噪音日志。
proxy_password 解密失败会阻止创建代理 dialer，不会静默直连。

诊断日志记录 rpc_code、rpc_type、error_type、error_summary，不记录敏感信息。

## 2026-06 MVP status addendum

- Basic realtime push is implemented through WebSocket and EventBus for `message.new`, `message.edited`, `message.deleted`, `dialog.upserted`, and sync/status events.
- The canonical logged-in entry is `/app/#/dashboard`; old Go Template dashboard is no longer the default logged-in landing page.
- Chat realtime deletion uses `telegram_message_ids` and does not include message body.
- Optimistic outgoing messages use local ids and are deduplicated against REST success and later WebSocket `message.new`.
- Dev publish is for local/manual verification only, disabled by default, protected by auth and CSRF, and does not access real Telegram.
- Before merging to `main`, real Telegram manual acceptance is still required: start `bin/atria.exe serve`, open `/app/#/chats`, confirm runtime live and `/api/realtime/ws` connected, receive a phone-sent message without refresh, verify non-current dialog preview/unread updates, verify outgoing messages do not duplicate, and confirm logs contain no message body or sensitive fields.

## 聊天页性能目标

### 加载时间约束

- 打开 `/app/#/chats` 后，会话列表**不能**长时间 skeleton
- 如果有缓存，会话列表应在 **300ms 到 800ms** 内显示
- 点击一个已有缓存的会话，最近消息应在 **300ms 到 800ms** 内显示
- 如果没有缓存或 Telegram 不可达，**10 到 15 秒内**显示明确错误，不允许无限 loading

### Cache-first 行为

1. **缓存有数据时立即返回**：有缓存时，不等待 runtime live，不等待 Telegram refresh，不因为 runtime connecting 而阻塞缓存返回
2. **缓存空时才尝试 Telegram/runtime**：缓存为空或 `force_refresh=true` 时才调 Telegram
3. **source 字段标识数据来源**：`cache`（仅缓存）、`telegram`（仅 Telegram）、`mixed`（两者合并）
4. **stale 字段标识数据时效**：`true` 表示数据来自缓存（可能过期）、`false` 表示实时数据
5. **Telegram refresh 失败不清空缓存**：刷新失败时保留现有缓存
6. **不允许无限 pending**：强制超时 15 秒

### Loading/Error/Empty/Stale 状态定义

| 状态 | 条件 | 行为 |
|------|------|------|
| loading（首次无数据） | `isLoading && allMessages.length === 0` | 显示 skeleton |
| stale cache | `isStale && isLoading` | 显示已有消息 + "正在刷新..." |
| error（首次无数据） | `error && allMessages.length === 0` | 显示 ErrorBanner + 重试 |
| empty | `!isLoading && allMessages.length === 0 && !error` | 显示空消息提示 |
| retrying（已有数据） | `isLoading && allMessages.length > 0` | 保留已有消息，不显示 skeleton |

### Runtime connecting 不阻塞阅读缓存

- `runtime connecting` 状态**不影响** dialogs/messages 加载
- 卡在 connecting 超过 15 秒，前端显示"连接较慢"
- connecting 时 REST 请求走临时客户端或缓存，不等待 executor
- `executor_ready=false` 时 tooltip 显示"实时通道尚未就绪，当前使用缓存/REST"

### `force_refresh` 参数

- 用户主动点击刷新按钮时，前端发送 `?force_refresh=true`
- 后端收到后跳过缓存，直接调 Telegram
- 用于用户想要获取最新数据的场景

## 2026-06 chat loading deadlock fix

- Chat page (`/app/#/chats`) must not show infinite skeleton. When runtime is `connecting`, REST dialogs/messages fall through to temporary client or cache instead of blocking on executor.
- Error states: dialogs and messages queries show `ErrorBanner` on failure with retry. Skeleton shows "加载时间较长" hint after 10 seconds.
- Cache-first: when cache has data, it is returned immediately regardless of runtime state. When cache is empty and Telegram is unreachable, a clear error is returned within the timeout (15s).
- Runtime status badge shows `last_error` tooltip and executor ready state for diagnostics.
- Frontend HTTP layer enforces 30-second fetch timeout via `AbortController`.

## 消息区排序规则

- **左侧会话列表**：按 `last_message_at DESC`（最新会话在顶部），pinned 优先。
- **右侧消息区**：按 `sent_at ASC`（旧消息在上，新消息在下）。
- 两套排序完全独立，不混用。
- 前端统一使用 `sortMessagesAsc()` 函数，使用 `Date.getTime()` 数值比较（避免 ISO 字符串精度差异导致错序）。
- `sent_at` 相同时按 `telegram_message_id ASC` 兜底。
- 实时新消息追加到底部（因为 sort ASC + 新消息 sent_at 最大）。
- older pagination 加载的历史消息插入 `olderPages` 顶部，合并后仍保持正序。
- force_refresh / reconcile merge 后仍保持正序。

## Emoji 支持

- 消息正文原样保留 Unicode emoji，不做替换。
- 预览截断使用 `safeTruncateText()`，基于 `Intl.Segmenter`（grapheme cluster）或 `Array.from`（code point），不截断 surrogate pair / ZWJ / 组合序列。
- Go 后端 `truncateText()` 使用 `[]rune` 切片，不按字节截断。
- CSS 字体栈包含 emoji fallback：`"Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji"`。
- 输入框支持原生 emoji 输入。
- `word-break: normal` 替代 `break-word`，避免拆断 ZWJ emoji 序列。

## 会话头像

- 当前使用 `AvatarInitials` 组件，显示名称首字符 fallback。
- 支持英文首字母、中文首字、数字、emoji 开头（grapheme 安全提取）。
- 空名称显示 `?`。
- 支持 `avatarUrl` 可选属性，有图片时显示图片，加载失败回退 fallback。
- 后端 Dialog DTO 有 `avatar_placeholder` 字段（首字母），预留 `avatar_url` 字段。
- 真实 Telegram 头像下载留作后续独立任务（需实现 photo 缓存和按需加载）。

## 会话唯一性（Canonical Peer Ref）

会话列表必须以 canonical `peer_ref` 保证唯一性，同一 Telegram 会话不允许显示多个条目。

### peer_ref 格式

| Telegram 类型 | 格式 | 示例 |
|--------------|------|------|
| User | `u_<telegram_user_id>` | `u_12345` |
| Chat (basic group) | `c_<telegram_chat_id>` | `c_67890` |
| Channel/Supergroup | `ch_<telegram_channel_id>` | `ch_11111` |

### 一致性要求

- Runtime mapper、ListDialogs mapper、message.new、dialog.upserted、delete update 必须使用同一 `mapPeerRef` 规则。
- `mapMessage()` 必须设置 `PeerRef`，确保 `upsertMessageCache` 和 `updateDialogPreview` 使用正确的 peer_ref。
- 不允许同一个 Telegram peer 生成不同格式的 peer_ref。
- 不允许用 username/title/access_hash 作为 peer_ref。

### 前端去重

- 前端 `ChatView.vue` 对 dialogs 按 `peer_ref` 防御性去重。
- `handleDialogUpserted` 忽略 `peer_ref` 为空的幽灵 dialog。
- 手动刷新和实时 upsert 使用同一 peer_ref 匹配逻辑。
- 不按 title 去重（不同会话可能同名）。

### 旧缓存修复

- Migration 10 清理 `PeerRef=""` 的幽灵记录。
- Migration 10 将 `ChatPeerCache` 唯一索引从全局改为 `(account_id, peer_ref)` 复合索引。
- 启动时自动执行迁移，无需手动干预。

## 实时状态反馈

- 后端服务断开时，badge 不能继续显示绿色"实时更新中"。
- Badge 综合 WebSocket 连接状态 + runtime 状态判断。
- 只有 WS connected + runtime live 时才显示绿色。
- 断线时保留本地缓存（dialogs/messages 不清空）。
- 刷新按钮点击时，如果后端不可达，应显示明确错误。
- 服务恢复后自动重连并恢复正确状态。
- 后端服务重启后，前端自动触发 runtime start，无需刷新浏览器。
- `ensureRuntimeStarted` 统一入口，带 8 秒防抖，防止重复 start。
- proxy_config_invalid / login_required / session_missing 不自动恢复。
- 详见 `docs/23_websocket_realtime_push.md`。

## 消息气泡 Sender Label

- 私聊（peer_type=user）：不显示 sender label，即使 sender_name 非空。
- 群聊/频道（peer_type=chat/channel）：incoming 消息且 sender_name 非空时显示 sender label。
- outgoing 消息：不显示 sender label。
- 判断逻辑：`showSenderLabel = !is_outgoing && sender_name && peerType !== 'user'`

## 消息区滚动行为

- 切换会话时默认滚到底部（最新消息）。
- 消息不足一屏时，CSS `margin-top: auto` 使消息靠底部显示，不出现突兀滚动条。
- 消息超过一屏时，`scheduleScrollToBottom` 定位到底部。
- scroll intent 状态机：stick-to-bottom / preserve-position / manual。
- scheduleScrollToBottom：nextTick → 双 rAF 确保 DOM 完成布局。
- ResizeObserver stick-to-bottom：补偿字体加载、emoji 渲染、reconcile merge 导致的 scrollHeight 变化。
- token 取消机制：旧 peer 的滚动任务不会影响新 peer。
- 实时新消息：nearBottom 时自动滚底，不在底部时显示"有新消息"提示。
- older pagination：MessageList 内部管理 anchor，handleScroll 触发时记录位置，消息变化后恢复。
- deleted/edited：不触发滚动。
- message-scroll-container 是唯一主滚动容器，body overflow:hidden 防止双滚动条。

## 滚动条样式

- 消息区和会话列表使用深色半透明滚动条。
- 宽度 5px，999px 圆角，hover 时增强可见性。
- scrollbar-gutter: stable 防止布局跳动。
- 支持 ::-webkit-scrollbar（Chrome/Edge/Safari）和 scrollbar-width/scrollbar-color（Firefox）。
