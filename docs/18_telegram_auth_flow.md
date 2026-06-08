# Telegram 认证流程与 Atria 登录状态机

## Telegram 用户登录状态机

```
[未认证]
    │
    ▼
auth.sendCode(phone, api_id, api_hash)
    │
    ├──→ auth.sentCode → [等待验证码]
    │         │
    │         ▼
    │    auth.signIn(phone, phone_code_hash, phone_code)
    │         │
    │         ├──→ auth.authorization → [登录成功]
    │         │
    │         ├──→ SESSION_PASSWORD_NEEDED (RPC 400) → [需要 2FA]
    │         │         │
    │         │         ▼
    │         │    account.getPassword() → 获取 SRP 参数
    │         │         │
    │         │         ▼
    │         │    auth.checkPassword(InputCheckPasswordSRP)
    │         │         │
    │         │         ├──→ auth.authorization → [登录成功]
    │         │         └──→ PASSWORD_HASH_INVALID → [密码错误，重试]
    │         │
    │         ├──→ PHONE_CODE_INVALID → [验证码错误，重试]
    │         ├──→ PHONE_CODE_EXPIRED → [验证码过期，重新 sendCode]
    │         └──→ auth.authorizationSignUpRequired → [未注册]
    │
    ├──→ SESSION_PASSWORD_NEEDED (future auth token + 2FA) → [需要 2FA]
    │
    └──→ 各种错误（PHONE_NUMBER_BANNED, FLOOD_WAIT 等）
```

## Atria 登录流程映射

| Atria 步骤 | Telegram 方法 | 说明 |
|---|---|---|
| StartLogin | auth.sendCode | 发送验证码，保存 phone_code_hash |
| SubmitCode | auth.signIn | 提交验证码，可能需要 2FA |
| SubmitPassword | account.getPassword + auth.checkPassword | SRP 密码验证 |
| completeLogin | — | 保存账号 session |

## SESSION_PASSWORD_NEEDED 含义

- Telegram RPC 错误码 400
- 表示该账号开启了 Two-Step Verification (2FA)
- 不是网络错误、不是代理错误、不是 API Key 错误、不是验证码错误
- 收到此错误后必须进入 SRP 密码验证流程

## SRP 密码验证流程

1. 调用 `account.getPassword()` 获取 SRP 参数（g, p, salt, srp_B, srp_id）
2. 客户端计算密码哈希（使用 gotd 的 `auth.PasswordHash` helper）
3. 调用 `auth.checkPassword(InputCheckPasswordSRP{id, A, M1})`
4. 成功返回 `auth.Authorization`，失败返回 `PASSWORD_HASH_INVALID`

## 安全要求

- **不得保存 2FA 密码**
- **不得记录 2FA 密码到日志**
- **不得记录 OTP 到日志**
- **不得记录 api_hash 到日志**
- **不得记录 proxy_password 到日志**
- **不得记录 phone_code_hash 明文到日志**

## Flow 中保存什么

| 字段 | 保存 | 说明 |
|---|---|---|
| FlowID | ✅ | 流程唯一标识 |
| PhoneEncrypted | ✅ | 加密手机号 |
| PhoneCodeHashEncrypted | ✅ | 加密的 phone_code_hash |
| APICredentialID | ✅ | API 凭据 ID |
| APIID | ✅ | Telegram API ID |
| State | ✅ | 当前状态（code_sent / waiting_password） |
| SessionStorageKey | ✅ | session 存储路径 |
| ExpiresAt | ✅ | 过期时间 |

| 字段 | 不保存 | 说明 |
|---|---|---|
| 明文手机号 | ❌ | 安全 |
| 明文 phone_code_hash | ❌ | 安全 |
| api_hash | ❌ | 安全 |
| 2FA 密码 | ❌ | 安全 |
| OTP | ❌ | 安全 |

## 常见 RPC 错误映射

| RPC 错误 | Atria ErrorKind | 用户提示 |
|---|---|---|
| PHONE_CODE_INVALID | code_invalid | 验证码错误，请检查后重新输入 |
| PHONE_CODE_EXPIRED | code_expired | 验证码已过期，请重新开始登录流程 |
| SESSION_PASSWORD_NEEDED | password_required | 该账号已开启两步验证，请输入 2FA 密码 |
| PASSWORD_HASH_INVALID | password_invalid | 2FA 密码错误，请重新输入 |
| AUTH_KEY_INVALID | session_context_lost | 登录会话上下文已丢失，请重新开始 |
| FLOOD_WAIT | flood_wait | 操作过于频繁，请等待 N 秒后重试 |
| API_ID_INVALID | api_key_invalid | Telegram API Key 不可用 |
| PHONE_NUMBER_INVALID | phone_invalid | 手机号无效 |

## gotd/td 源码阅读证据

**版本**：github.com/gotd/td v0.115.0

**已读取的本地源码文件**：
- `telegram/auth/user.go` — `Client.Password()` 方法、`Client.SignIn()` 方法、`ErrPasswordAuthNeeded`、`ErrPasswordInvalid`
- `telegram/auth/password.go` — `PasswordHash()` 函数（SRP 哈希计算）
- `telegram/auth/flow.go` — `Flow.Run()` 认证流程
- `telegram/auth/client.go` — `Client` 结构体
- `tgerr/error.go` — `tgerr.Error` RPC 错误类型、`tgerr.As()` 解包
- `tg/tl_errors_gen.go` — `tg.IsPasswordHashInvalid()` 错误检查

**关键结论**：
1. gotd `auth.Client.Password()` 使用 `p.CurrentAlgo`（当前密码算法），不是 `p.NewAlgo`（设置新密码算法）
2. `PasswordHash()` 接收 `PasswordKdfAlgoClass` 接口，内部断言为 `*PasswordKdfAlgoSHA256SHA256PBKDF2HMACSHA512iter100000SHA256ModPow`
3. `auth.Client.SignIn()` 遇到 `SESSION_PASSWORD_NEEDED` 返回 `ErrPasswordAuthNeeded`
4. `auth.Client.Password()` 遇到 `PASSWORD_HASH_INVALID` 返回 `ErrPasswordInvalid`
5. `ErrPasswordInvalid` 注释明确说明：Telegram 默认不 trim 密码空白字符，需要 `strings.TrimSpace`

**Atria 2FA 实现**：
- 使用 `auth.PasswordHash()` gotd helper 计算 SRP 哈希
- 使用 `tg.AuthCheckPassword` 提交验证
- 使用 `passwordInfo.CurrentAlgo`（修复后，之前错误使用了 `NewAlgo`）
- 对密码做 `strings.TrimSpace`

## 后续开发要求

1. **修改登录链路前必须先看本文件和 Telegram 官方 Auth 文档**
2. **不允许把未知 Telegram RPC 错误粗暴归类为 network_error**
3. **不允许新增登录状态但不补状态机文档和测试**
4. **SESSION_PASSWORD_NEEDED 不是错误，是状态转换信号**
5. **2FA 密码必须使用 `CurrentAlgo`，不是 `NewAlgo`**
6. **不允许手写未验证的 SRP，必须使用 gotd `auth.PasswordHash` helper**
