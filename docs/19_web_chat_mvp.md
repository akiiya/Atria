# Web 聊天 MVP

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

## 安全日志

- 不记录完整消息正文（只记录 text_len）
- 不记录 access_hash 明文
- 不记录 api_hash
- 不记录 proxy_password
- 不记录 session path

## 路由

| 路由 | 方法 | 说明 |
|------|------|------|
| `/chats` | GET | 会话列表页面 |
| `/chats/:peer_ref` | GET | 消息历史页面 |
| `/api/chats/:peer_ref/messages` | POST | 发送消息（JSON） |

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
