# Atria — 架构设计

## 1. 总体架构

Atria 采用单体单二进制架构：

```
┌─────────────────────────────────────────────────┐
│                   Atria 二进制                    │
│                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │   CLI    │  │  Server  │  │  MTProto     │   │
│  │  (cmd)   │──│  (Gin)   │  │  Client      │   │
│  └──────────┘  └────┬─────┘  │  (gotd/td)   │   │
│                     │        └──────┬───────┘   │
│              ┌──────┴──────┐        │           │
│              │   Router    │        │           │
│              │  (handlers) │        │           │
│              └──────┬──────┘        │           │
│                     │               │           │
│  ┌──────────────────┼───────────────┼────────┐  │
│  │            中间件层              │         │  │
│  │  ┌─────┐ ┌─────┐ ┌─────┐       │         │  │
│  │  │Auth │ │CSRF │ │Log  │       │         │  │
│  │  └─────┘ └─────┘ └─────┘       │         │  │
│  └──────────────────┼───────────────┼────────┘  │
│                     │               │           │
│              ┌──────┴──────┐        │           │
│              │   Models    │        │           │
│              │   (GORM)    │        │           │
│              └──────┬──────┘        │           │
│                     │               │           │
│  ┌──────────────────┼───────────────┼────────┐  │
│  │            安全层                │         │  │
│  │  ┌───────┐ ┌───────┐ ┌────────┐ │         │  │
│  │  │Crypto │ │Secret │ │Audit   │ │         │  │
│  │  │AES-GCM│ │Key    │ │Filter  │ │         │  │
│  │  └───────┘ └───────┘ └────────┘ │         │  │
│  └──────────────────┼───────────────┼────────┘  │
│                     │               │           │
│              ┌──────┴──────┐  ┌─────┴───────┐   │
│              │  Database   │  │  Sessions    │   │
│              │  (SQLite)   │  │  (加密文件)   │   │
│              └─────────────┘  └─────────────┘   │
│                                                  │
│  ┌──────────────────────────────────────────┐   │
│  │  嵌入式 Web 资源 (go:embed)              │   │
│  │  templates/ + static/                    │   │
│  └──────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

## 2. 模块结构

```
atria/
├── cmd/atria/              # 入口点，CLI 调度
│   └── main.go
├── internal/
│   ├── config/             # 配置加载与校验
│   │   ├── config.go
│   │   └── config_test.go
│   ├── crypto/             # 加密工具
│   │   ├── secret.go       # 密钥管理
│   │   ├── aesgcm.go       # AES-256-GCM 加解密
│   │   ├── fingerprint.go  # 指纹计算
│   │   └── *_test.go
│   ├── security/           # 敏感数据加密辅助
│   │   └── sensitive.go
│   ├── database/           # 数据库初始化与迁移
│   │   └── database.go
│   ├── model/              # GORM 数据模型
│   │   ├── admin.go
│   │   ├── api_credential.go
│   │   ├── audit_log.go
│   │   ├── system_setting.go
│   │   └── telegram_account.go
│   ├── auth/               # Web 认证
│   │   ├── session.go      # Session 数据结构
│   │   ├── cookie.go       # Cookie Session 编解码
│   │   ├── csrf.go         # CSRF 保护
│   │   ├── middleware.go   # 认证中间件
│   │   └── *_test.go
│   ├── audit/              # 审计日志
│   │   ├── audit.go
│   │   └── audit_test.go
│   ├── server/             # HTTP 服务器
│   │   ├── server.go
│   │   ├── router.go
│   │   ├── view.go         # 统一 ViewData
│   │   └── response.go     # 统一错误处理
│   ├── updater/            # 自更新接口（预留）
│   │   ├── updater.go
│   │   └── types.go
│   ├── version/            # 版本信息
│   │   └── version.go
│   └── web/                # 嵌入式 Web 资源访问
│       └── embed.go
├── web/
│   ├── templates/          # HTML 模板（嵌入）
│   └── static/             # CSS, JS（嵌入）
└── docs/                   # 文档
```

### 模块职责

| 模块 | 职责 |
|------|------|
| `cmd/atria` | CLI 解析、命令调度、顶层初始化 |
| `internal/config` | 配置加载、校验、目录初始化 |
| `internal/crypto` | 密钥管理、AES-256-GCM 加解密、指纹计算 |
| `internal/security` | API Hash、手机号、Session 数据的加密辅助 |
| `internal/database` | 数据库连接初始化、驱动抽象、自动迁移 |
| `internal/model` | GORM 模型定义、字段约束 |
| `internal/auth` | Web Session 编解码、CSRF 保护、认证中间件 |
| `internal/audit` | 审计日志写入、敏感字段过滤 |
| `internal/server` | HTTP 服务器、路由、ViewData、错误处理 |
| `internal/updater` | 自更新接口预留 |
| `internal/version` | 版本信息（支持 ldflags 注入） |
| `internal/web` | go:embed 声明和模板解析 |

## 3. 配置校验

配置加载流程：

```
1. 加载默认值
    │
2. 读取环境变量覆盖
    │
3. 调用 Validate() 校验
    │
    ├── Host 非空
    ├── Port 在 1-65535
    ├── DataDir / SessionDir / LogDir 非空
    ├── DatabaseDriver 在允许列表中
    ├── CookieName 非空
    ├── CookieSameSite 在 lax/strict/none 中
    ├── SessionTTL > 0
    └── CSRFHeaderName / CSRFFieldName 非空
    │
4. 调用 EnsureDirs() 创建目录
```

## 4. 数据库 AutoMigrate

启动时自动迁移所有模型：

```
database.Init(driver, dsn)
    │
    ├── 选择驱动（sqlite/postgres/mysql）
    ├── 建立连接
    │
database.AutoMigrate(db)
    │
    ├── Admin
    ├── APICredential
    ├── TelegramAccount
    ├── AccountSession
    ├── AccountSyncSnapshot
    ├── AuditLog
    └── SystemSetting
```

TODO: 正式版本可能需要版本化迁移（如 golang-migrate）。

## 5. 加密工具层

```
┌─────────────────────────────────────────┐
│           加密工具层次结构               │
├─────────────────────────────────────────┤
│                                         │
│  internal/crypto（底层工具）             │
│  ├── secret.go    — 密钥加载/生成       │
│  ├── aesgcm.go    — AES-256-GCM 加解密  │
│  └── fingerprint.go — SHA-256 指纹      │
│                                         │
│  internal/security（业务辅助）           │
│  └── sensitive.go — API Hash/Phone/     │
│                     Session 加密辅助     │
│                                         │
│  internal/auth（Web Session）            │
│  ├── cookie.go    — Session token 编解码│
│  └── csrf.go      — CSRF token 生成     │
│                                         │
└─────────────────────────────────────────┘
```

### 密钥管理

- 环境变量 `ATRIA_SECRET_KEY`（base64 编码的 32 字节密钥）
- 密钥文件 `data/secret.key`（base64 格式，权限 0600）
- 不存在时自动生成
- 所有加密操作使用同一密钥，通过 AAD 区分数据类型

### AES-256-GCM

- 使用 `crypto/aes` + `cipher.NewGCM`
- 随机 nonce（12 字节）
- 输出格式：`nonce + ciphertext + tag`
- 支持 AAD（附加认证数据）

### AAD 常量

| 用途 | AAD |
|------|-----|
| API Hash | `atria:api_hash:v1` |
| 手机号 | `atria:phone:v1` |
| Session 数据 | `atria:session:v1` |
| Web Session | `atria:web_session:v1` |

## 6. Web Session

```
登录流程（Phase 2 实现）：
    用户提交用户名 + 密码
    │
    ├── 验证密码（bcrypt/argon2id）
    │
    ├── 创建 SessionClaims
    │   ├── AdminID
    │   ├── Username
    │   ├── IssuedAt
    │   └── ExpiresAt
    │
    ├── SessionClaims → JSON → AES-GCM 加密 → base64
    │
    └── 设置 Cookie
        ├── Name: atria_session
        ├── HttpOnly: true
        ├── Secure: (根据配置)
        └── SameSite: (根据配置)
```

### 认证中间件

- 从 Cookie 读取 token
- AES-GCM 解密 + JSON 反序列化
- 检查 ExpiresAt 过期
- 成功：将 AdminID/Username 写入 gin.Context
- 失败：返回 401 JSON

## 7. CSRF 保护

```
POST / PUT / PATCH / DELETE 请求
    │
    ├── 从 Header (X-CSRF-Token) 或表单 (csrf_token) 读取 token
    │
    ├── 与预期 token 比对
    │
    ├── 匹配 → 继续处理
    └── 不匹配 → 返回 403
```

- GET / HEAD / OPTIONS 不校验
- token 由 `GenerateCSRFToken()` 生成（32 字节随机数，base64 URL 编码）
- 本轮只实现中间件，不强制应用到所有路由

## 8. 审计日志

### 写入流程

```
audit.Log(ctx, db, Event{...})
    │
    ├── filterMetadata(metadata) — 过滤敏感字段
    │   ├── password → ***REDACTED***
    │   ├── api_hash → ***REDACTED***
    │   ├── session → ***REDACTED***
    │   └── ...
    │
    ├── 转换为 JSON
    │
    └── 写入 audit_logs 表
```

### 敏感字段过滤

匹配规则：key 名称包含敏感关键词（大小写不敏感）。

敏感关键词：password、api_hash、session、token、code、two_factor、secret、csrf_token、cookie、authorization。

## 9. 统一 ViewData

所有页面使用 `ViewData` 结构渲染模板：

```go
type ViewData struct {
    Title                   string
    ActiveNav               string
    Version                 string
    Commit                  string
    BuildDate               string
    Theme                   string
    CSRFToken               string
    CurrentCredentialName   string
    CurrentCredentialMasked string
    Flash                   string
    Error                   string
    DatabaseDriver          string
    DatabaseDSN             string
    DataDir                 string
    SessionDir              string
    LogDir                  string
    ListenAddr              string
    Data                    any
}
```

- 不在模板中硬编码版本号
- 不在模板中硬编码敏感信息
- DSN 使用脱敏版本

## 10. 统一错误处理

```
请求进入
    │
    ├── isHTMLRequest? ──Yes──→ RenderError() → 错误页面 HTML
    │
    └── No──→ JSONError() → JSON 错误响应

内部错误：
    LogAndError() → 日志记录内部错误 + 返回用户友好消息
```

- 不暴露内部错误堆栈
- 日志可记录内部错误，但不包含敏感信息

## 11. MTProto 登录流程（Phase 4.1.1 已实现）

Atria 实现三阶段 MTProto 用户登录流程：

```
用户输入手机号
    │
    ▼
StartLogin ──────────────────────────────┐
    │                                     │
    ├── 发送验证码 (auth.sendCode)         │
    ├── 保存加密 phone_code_hash          │
    ├── 保存临时 gotd Session             │
    └── 返回 LoginStateCodeSent           │
                                          │
用户输入验证码 ◄──────────────────────────┘
    │
    ▼
SubmitCode ──────────────────────────────┐
    │                                     │
    ├── 提交验证码 (auth.signIn)           │
    ├── 成功 → 获取账号资料               │
    │         保存正式加密 Session         │
    │         创建 telegram_accounts      │
    │         删除临时 Session            │
    │         返回 LoginStateAuthorized   │
    │                                     │
    └── 需要 2FA → 返回 LoginStateWaitingPassword
                                          │
用户输入 2FA 密码 ◄──────────────────────┘
    │
    ▼
SubmitPassword ──────────────────────────┐
    │                                     │
    ├── 提交 2FA 密码                     │
    ├── 成功 → 获取账号资料               │
    │         保存正式加密 Session         │
    │         创建 telegram_accounts      │
    │         删除临时 Session            │
    │         返回 LoginStateAuthorized   │
    └── 失败 → 返回错误
```

### 临时 Session 设计

登录过程中需要保存临时 gotd Session：
- 位置：`data/sessions/tmp/`
- 文件名：`login_flow_{flow_id}.enc`
- 加密方式：AES-256-GCM
- 生命周期：与 LoginFlow 一致
- 清理时机：登录成功、失败或过期时删除

### 正式 Session 保存

登录成功后保存正式 Session：
- 位置：`data/sessions/session_{account_id}.enc`
- 加密方式：AES-256-GCM
- 数据库只保存路径和指纹

### Web Handler 与 MTProto Client 接口

```
Web Handler (account_handler.go)
    │
    ├── 校验 CSRF、手机号、凭据
    │
    ├── 调用 MTProto Client interface
    │   ├── StartLogin
    │   ├── SubmitCode
    │   └── SubmitPassword
    │
    ├── 处理返回结果
    │   ├── LoginStateCodeSent → 重定向验证码页
    │   ├── LoginStateWaitingPassword → 重定向 2FA 页
    │   └── LoginStateAuthorized → 创建账号、重定向详情页
    │
    └── 写审计日志
```

## 12. Session 生命周期架构

```
登录成功
    │
    ▼
保存正式加密 Session
    │   data/sessions/session_{account_id}.enc
    │   数据库：account_sessions（路径、指纹、状态=active）
    │
    ▼
同步资料（可选）
    │   调用 SyncProfile
    │   更新 telegram_accounts
    │   写入 account_sync_snapshots
    │
    ▼
检测 Session（可选）
    │   调用 CheckSession
    │   更新 account_sessions.status
    │   更新 account_sessions.last_verified_at
    │
    ▼
┌─────────────────────────────────────┐
│          Session 结束方式            │
├──────────────────┬──────────────────┤
│   远端 Logout    │  本地删除 Session │
├──────────────────┼──────────────────┤
│ 调用 auth.logOut │ 不调用 Telegram   │
│ 删除本地文件     │ 删除本地文件      │
│ 远端 Session 失效│ 远端可能仍有效    │
│ 状态→logged_out  │ 状态→deleted      │
└──────────────────┴──────────────────┘
    │
    ▼
Session invalid / deleted / error
    │
    ▼
重新登录引导
```

### AccountService 方法职责

| 方法 | 职责 |
|------|------|
| `SyncProfile` | 调用 MTProto 同步资料，更新数据库，写审计日志 |
| `CheckSession` | 调用 MTProto 检测状态，更新 session status，写审计日志 |
| `RemoteLogout` | 调用 MTProto 远端注销，成功后删除本地文件，写审计日志 |
| `DeleteLocalSession` | 仅删除本地文件，不调用 Telegram，写审计日志 |
| `HandleSessionInvalid` | 更新 session/account 状态为 invalid/error |
| `CleanupExpiredLoginFlows` | 清理过期 LoginFlow 和临时 Session 文件 |

### Session 状态定义

**telegram_accounts.status：**
- `active` — 账号正常，Session 有效
- `invalid` — Session 失效，需要重新登录
- `logged_out` — 已登出（远端或本地）
- `restricted` — 账号受限
- `error` — 检测异常

**account_sessions.status：**
- `active` — Session 文件有效
- `invalid` — Session 远端失效
- `deleted` — Session 文件已删除
- `error` — Session 文件缺失或解密失败
