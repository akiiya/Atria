# Atria — Telegram API Research

> **Note:** This document is based on architectural requirements and general MTProto knowledge. It requires manual verification against official Telegram documentation and the gotd/td library documentation before production use.

## 1. Telegram Bot API vs MTProto User API

| Aspect | Bot API | MTProto User API |
|--------|---------|------------------|
| Protocol | HTTP/HTTPS REST | MTProto (binary, TCP) |
| Account Type | Bot accounts only | User accounts |
| Authentication | Bot token | Phone number + code + optional 2FA |
| Session | Stateless (token-based) | Stateful (session file) |
| Capabilities | Limited (no user login, no full contact access) | Full user account access |
| Rate Limits | Per-bot, documented | Per-account, FLOOD_WAIT mechanism |
| Library Ecosystem | Extensive (all languages) | Fewer mature libraries |

**Why MTProto for Atria:**

Atria manages **user sessions**, not bots. The MTProto user API is the only way to authenticate and manage real Telegram user accounts. The Bot API does not support user login, contact management, or the session-based operations Atria requires.

## 2. gotd/td Library Assessment

**Repository:** https://github.com/gotd/td

**Key characteristics:**
- Pure Go implementation of MTProto
- Auto-generated from Telegram's TL schema
- Provides low-level MTProto client and higher-level helpers
- Active maintenance (as of last known state)
- Supports session management, 2FA, and file operations

**Strengths:**
- Native Go, no CGo dependencies
- Comprehensive TL schema coverage
- Built-in session handling
- Supports both CDN and direct download

**Considerations:**
- API surface is large; subset usage is recommended
- FLOOD_WAIT handling must be implemented by the caller
- Session serialization format needs to be understood for encrypted storage

**Assessment:** gotd/td is the recommended library for Atria. It covers the required capabilities and has a mature codebase.

## 3. Login Flow Planning

The MTProto user login flow:

1. **Send Code Request** — Provide phone number, receive code via SMS/Telegram
2. **Submit Code** — Enter received code to authenticate
3. **Handle 2FA** — If two-factor authentication is enabled, prompt for password
4. **Session Persistence** — Save the authenticated session for future use

**Implementation notes:**
- Each step requires user interaction (no fully automated login)
- Phone number must be provided by the user
- Verification code delivery is handled by Telegram (SMS, Telegram app, or call)
- 2FA password is user-provided, never stored by Atria
- Session must be encrypted before storage

## 4. Session Management Planning

**Session file:**
- MTProto sessions are stateful; the client maintains a session file
- Contains auth key, server salts, sequence numbers, and message IDs
- Loss of session file requires re-authentication

**Atria's approach:**
- Session files stored in `data/sessions/`
- File names use account ID or UUID (no phone numbers)
- Encrypted with the local secret key before writing to disk
- Database stores only the path, fingerprint, and status

**Encryption versioning:**
- `encryption_version` field allows future key rotation
- Version 1: AES-256-GCM with key from `data/secret.key`

## 5. 2FA Handling

- 2FA password is provided by the user during login
- Passed directly to gotd/td for authentication
- **Never stored** by Atria (not in database, not in logs, not in session file)
- If 2FA is required, the UI must prompt the user

## 6. FLOOD_WAIT / Rate Limiting

Telegram may return `FLOOD_WAIT` errors when rate limits are exceeded.

**Strategy:**
- Parse `FLOOD_WAIT` error to extract wait duration
- Implement exponential backoff for retries
- Log wait events (without sensitive context)
- Surface wait status to the UI so users understand delays
- Do not attempt to bypass or circumvent rate limits

## 7. 当前实现状态

### 已实现能力（Phase 4.3）

**MTProto 登录三阶段：**
- StartLogin：调用 auth.sendCode 发送验证码
- SubmitCode：调用 auth.signIn 提交验证码
- SubmitPassword：调用 auth.checkPassword + SRP 处理两步验证

**账号资料同步：**
- 基于已保存的加密 Session 同步 Telegram 账号基础资料
- 支持 user_id、phone、username、first_name、last_name、is_premium 等字段

**Session 状态检测：**
- 检测已保存 Session 是否仍然有效
- 支持 active、invalid、logged_out、error 状态

**真实远端 Logout：**
- 调用 auth.logOut 注销 Telegram 远端 Session
- 成功后删除本地加密 Session 文件
- 失败时不删除本地 Session（如 FLOOD_WAIT）

**Session 生命周期管理：**
- 登录成功 → 加密保存 Session → 同步资料 → 检测状态 → 远端 Logout 或本地删除 → Session 失效 → 重新登录引导

### 待实现能力

- 群组/频道列表同步（Phase 5）
- 群组/频道元数据同步（Phase 5）
- 文件上传/下载（Phase 5+）
- 消息读取（Phase 5+，严格风控）

### Session invalid / unauthorized 处理策略

- Session 文件不存在：状态设为 error，提示重新登录
- Session 解密失败：状态设为 error，提示检查密钥或重新登录
- 远端 Session 无效：状态设为 invalid，提示重新登录
- FLOOD_WAIT：不改变状态，显示等待时间，不自动重试

## 8. Not Implementable / Prohibited

These capabilities are explicitly prohibited:

- Sending messages in bulk
- Inviting users to groups
- Joining/leaving groups automatically
- Scraping user data at scale
- Bypassing FLOOD_WAIT or other Telegram limits
- Automated account creation
- Phone number verification automation

## 9. References

- gotd/td: https://github.com/gotd/td
- Telegram MTProto documentation: https://core.telegram.org/mtproto
- Telegram API documentation: https://core.telegram.org/api
- TL schema: https://github.com/gotd/tl
