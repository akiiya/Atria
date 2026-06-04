# Atria — 安全策略

## 1. 密码哈希

**算法：** bcrypt（默认）或 argon2id（可配置）

**实现：**
- 管理员密码在存储前进行哈希处理
- `password_hash` 字段存储哈希值
- `password_algo` 字段记录使用的算法
- 明文密码永不存储、记录或返回在 API 响应中
- 密码验证使用对应库的比较函数

**配置：**
- 默认：bcrypt cost factor 12
- 备选：argon2id，内存 64MB，迭代 3，并行度 4

## 2. AES-256-GCM 加密

Atria 使用 AES-256-GCM 作为核心加密算法，用于保护所有敏感数据。

**实现位置：** `internal/crypto/aesgcm.go`

**加密流程：**
```
明文数据
    │
    ├── 生成 12 字节随机 nonce（crypto/rand）
    │
    ├── AES-256-GCM 加密
    │   ├── key: 32 字节密钥
    │   ├── plaintext: 明文数据
    │   ├── nonce: 随机值
    │   └── aad: 附加认证数据
    │
    └── 输出：nonce + ciphertext + tag
```

**AAD（附加认证数据）：**
- 用于区分不同类型的加密数据
- API Hash: `atria:api_hash:v1`
- 手机号: `atria:phone:v1`
- Session 数据: `atria:session:v1`
- Web Session: `atria:web_session:v1`

**安全要求：**
- 错误信息不包含明文或密文内容
- nonce 使用 `crypto/rand` 生成
- 密钥长度固定 32 字节

## 3. 密钥管理

**密钥来源（按优先级）：**
1. 环境变量 `ATRIA_SECRET_KEY`（base64 或 hex 编码的 32 字节密钥）
2. 密钥文件 `data/secret.key`（base64 格式）
3. 自动生成并保存到文件

**密钥文件：**
- 格式：base64 编码的 32 字节随机密钥
- 权限：0600（仅所有者可读写）
- 路径可通过 `ATRIA_SECRET_KEY_FILE` 配置

**密钥备份：**
- `secret.key` 是所有加密数据的唯一解密密钥
- 丢失密钥 = 所有加密数据无法恢复
- 用户必须将 `secret.key` 备份到安全位置
- README 和文档必须明确提示备份重要性

**密钥轮换：**
- `encryption_version` 字段支持未来密钥轮换
- 当前版本：v1，使用单一密钥
- 未来版本可使用新密钥重新加密

## 4. API Hash 保护

**存储：**
- API Hash 在应用层加密后存入数据库
- `encrypted_api_hash` 字段存储密文
- `api_hash_fingerprint` 字段存储展示用指纹

**加密辅助函数：** `internal/security/sensitive.go`
```go
func EncryptAPIHash(key []byte, apiHash string) (encrypted, fingerprint string, err error)
func DecryptAPIHash(key []byte, encrypted string) (string, error)
```

**展示规则：**
- 列表页只显示指纹
- 编辑页不允许回显完整 API Hash
- API 响应通过 `json:"-"` 排除密文字段

**日志要求：**
- API Hash 永不写入日志
- 审计日志 metadata 不得包含 API Hash

## 5. 手机号保护

**加密辅助函数：** `internal/security/sensitive.go`
```go
func EncryptPhone(key []byte, phone string) (encrypted, fingerprint string, err error)
func DecryptPhone(key []byte, encrypted string) (string, error)
```

**存储：**
- 手机号加密后存入数据库
- `phone_encrypted` 字段存储密文
- `phone_fingerprint` 字段存储指纹

**日志要求：**
- 手机号永不写入日志

## 6. Session 数据加密

**加密辅助函数：** `internal/security/sensitive.go`
```go
func EncryptSessionData(key []byte, data []byte) ([]byte, error)
func DecryptSessionData(key []byte, encrypted []byte) ([]byte, error)
```

**存储：**
- MTProto Session 数据加密后写入文件
- 文件名使用 account_id 或 UUID（不包含手机号）
- 存储在 `data/sessions/` 目录
- 数据库只存储文件路径、指纹和加密版本

## 7. Web Cookie Session

**实现位置：** `internal/auth/cookie.go`

**Session 数据结构：**
```go
type SessionClaims struct {
    AdminID   uint
    Username  string
    IssuedAt  time.Time
    ExpiresAt time.Time
}
```

**加密流程：**
```
SessionClaims
    │
    ├── JSON 序列化
    │
    ├── AES-256-GCM 加密（AAD: atria:web_session:v1）
    │
    └── base64 编码 → Cookie token
```

**Cookie 属性：**
- `HttpOnly: true` — 不可被 JavaScript 访问
- `Secure: true` — 仅 HTTPS 传输（根据配置）
- `SameSite: lax/strict/none`（根据配置）
- `Path: /` — 全站有效
- `MaxAge` — 可配置的 Session 超时（默认 24 小时）

**安全要求：**
- token 中不包含明文用户名
- 过期 token 自动失效
- 登出清除 Cookie

## 8. CSRF 防护

**实现位置：** `internal/auth/csrf.go`

**Token 生成：**
- 使用 `crypto/rand` 生成 32 字节随机数
- base64 URL 安全编码

**校验规则：**
- GET / HEAD / OPTIONS 不校验
- POST / PUT / PATCH / DELETE 需要校验
- token 从 Header (`X-CSRF-Token`) 或表单字段 (`csrf_token`) 读取
- 校验失败返回 403

**配置：**
- 可通过 `ATRIA_CSRF_ENABLED` 启用/禁用
- Header 名称和表单字段名称可配置

## 9. 审计日志脱敏

**实现位置：** `internal/audit/audit.go`

**敏感字段过滤：**
- 自动过滤 Metadata 中的敏感字段
- 匹配规则：key 名称包含敏感关键词（大小写不敏感）
- 敏感字段值替换为 `***REDACTED***`

**敏感关键词列表：**
- password
- password_hash
- api_hash
- session
- token
- code
- two_factor
- secret
- secret_key
- csrf_token
- cookie
- authorization

**示例：**
```go
// 输入
metadata := map[string]any{
    "username": "admin",
    "password": "secret123",
    "api_hash": "abc123",
}

// 输出
filtered := map[string]any{
    "username": "admin",
    "password": "***REDACTED***",
    "api_hash": "***REDACTED***",
}
```

## 10. 风险策略系统

每个 API 凭据有独立的风险策略：

| 策略 | 行为 |
|------|------|
| `disabled` | 禁止高风险操作（默认） |
| `enabled` | 允许高风险操作 |
| `confirm` | 需要用户明确确认 |

**高风险操作包括：**
- 批量消息或发送
- 批量邀请成员
- 批量加入或退出群组
- 自动化账号操作
- 可能触发 Telegram 速率限制或封禁的操作

**执行流程：**
- `disabled` → 操作拒绝 + 审计日志
- `enabled` → 操作执行 + 审计日志
- `confirm` → 提示确认 + 审计日志

## 11. 速率限制

**应用层：**
- 登录尝试：可配置的 IP 限制（默认：15 分钟 5 次）
- API 请求：可配置的 Session 限制（默认：1 分钟 100 次）
- MTProto 操作：遵循 Telegram 的 FLOOD_WAIT 响应

**实现：**
- Gin 中间件实现 HTTP 速率限制
- FLOOD_WAIT 错误使用指数退避
- 速率限制事件记录到审计日志

## 12. 输入验证

**服务端：**
- 所有用户输入在处理前验证
- 字符串长度限制为模型字段大小
- 数值范围验证
- GORM 参数化查询防止 SQL 注入
- Go html/template 自动转义防止 XSS

**文件上传：**
- Phase 1 无文件上传功能
- 后续添加时：文件类型验证、大小限制、内容扫描

## 13. 备份与恢复

**需要备份的内容：**
- `data/atria.db` — 数据库（所有元数据）
- `data/secret.key` — 加密密钥（Session 恢复必需）
- `data/sessions/` — 加密的 Session 文件

**恢复场景：**
- 丢失 `secret.key`：Session 文件无法解密，所有账号需要重新登录
- 丢失数据库：从备份恢复，如果 `secret.key` 保留则 Session 文件有效
- 丢失 Session 文件：账号需要重新登录，数据库元数据保留

## 14. 禁止用途

Atria **不是**以下用途的工具：

- 垃圾信息或未经请求的消息
- 批量邀请成员或加入群组
- 账号养殖或培育
- 手机号采集或验证码平台
- 账号交易或市场操作
- 平台限制绕过
- 粉丝、浏览量或互动量膨胀
- 自动化骚扰
- 违反 Telegram 服务条款的任何活动

## 15. 漏洞报告

如果发现 Atria 的安全漏洞：

1. 不要公开 issue
2. 私下报告给维护者
3. 包含描述、复现步骤和潜在影响
4. 在公开披露前给予合理的修复时间

## 16. 安全更新

- 安全补丁优先于 feature 开发
- 通过 GitHub releases 通知用户安全更新
- 关键漏洞立即发布补丁版本

## 17. MTProto 登录安全策略

### api_hash 处理
- 从数据库读取后立即解密
- 仅短暂存在于内存中
- 不写入日志
- 不写入审计 metadata

### 手机号处理
- 使用 AES-256-GCM 加密存储
- 日志和审计只记录脱敏版本
- 不在页面显示完整手机号

### 验证码处理
- 不保存到数据库
- 不写入日志
- 不写入审计 metadata
- 仅用于提交给 Telegram 服务器

### 2FA 密码处理
- 不保存到数据库
- 不写入日志
- 不写入审计 metadata
- 仅用于提交给 Telegram 服务器

### phone_code_hash 处理
- 加密存储在内存 FlowStore 中
- 不写入日志
- 不持久化到数据库
- 仅用于验证码提交

### 临时 Session 处理
- 加密存储在 `data/sessions/tmp/`
- 使用 AES-256-GCM 加密
- 登录完成或过期后删除
- 不写入日志

### 正式 Session 处理
- 加密存储在 `data/sessions/`
- 使用 AES-256-GCM 加密
- 数据库只保存路径和指纹
- 不保存明文 Session 到数据库

### FLOOD_WAIT 处理
- 解析等待时间
- 显示给用户
- 不自动重试
- 不绕过限制

### 远端 Logout 安全策略
- 调用 MTProto auth.logOut 注销远端 Session
- 成功后删除本地加密 Session 文件
- 失败时不删除本地 Session（如 FLOOD_WAIT、网络错误）
- 需要服务端确认字段（confirm=remote_logout）
- 写入审计日志 account.remote_logout 或 account.remote_logout_failed

### 本地删除 Session 安全策略
- 仅删除本地加密 Session 文件
- 不调用 Telegram 远端
- Telegram 远端 Session 可能仍然有效
- 需要服务端确认字段（confirm=delete_local_session）
- 写入审计日志 account.local_session_deleted
- 不删除账号基础资料

### Session 失效处理策略
- Session 文件不存在：状态设为 error，提示重新登录
- Session 解密失败：状态设为 error，提示检查密钥或重新登录
- 远端 Session 无效：状态设为 invalid，提示重新登录
- FLOOD_WAIT：不改变状态，显示等待时间，不自动重试
- secret.key 丢失或更换：旧 Session 无法解密，需要重新登录

### 手动验证注意事项
- 真实登录需要有效 API 凭据
- 不要在测试中使用真实凭据
- 不要提交真实凭据到代码仓库
- 参见 docs/09_manual_mtproto_login_test.md

## 18. Web 自更新安全策略

### 更新源限制
- 只允许从配置的 GitHub 仓库 Release 下载
- 不允许用户在 UI 中输入任意下载 URL
- 生产环境只使用 UpdateRepo 配置生成 GitHub API URL
- 测试环境可使用 UpdateCheckURL 指向 mock server

### checksum 校验
- 默认强制校验 SHA-256 checksum
- checksum 不匹配必须失败
- 不允许跳过 checksum（除非显式测试模式）

### 不执行远程脚本
- 只下载二进制文件，不执行任何脚本
- 不运行下载内容中的代码
- 不允许远程代码执行

### Docker 环境
- Docker 环境下禁用 ApplyUpdate
- CheckLatest 和 DownloadUpdate 仍然可用
- 提示使用新镜像重建容器

### 失败回滚
- 更新前备份当前二进制
- 替换失败时自动恢复备份
- 用户可手动从备份目录恢复

### 数据保护
- 不删除 data/ 目录
- 不覆盖 secret.key
- 不删除 Session 文件
- 不修改业务数据

### 审计日志
- 所有更新操作记录审计日志
- 审计 metadata 不包含敏感信息
- 记录 current_version、latest_version、asset_name、status

### 测试要求
- 普通测试不访问真实 GitHub Release
- 测试使用 mock HTTP server
- 自更新 Try-Run 使用 tmp/ 沙箱
- 不写入真实系统路径
