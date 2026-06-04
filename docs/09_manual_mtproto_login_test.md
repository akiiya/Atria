# 手动 MTProto 登录测试指南

本文档说明如何在本地手动验证 Atria 的完整 MTProto 登录和 Session 生命周期。

## A. 环境准备

### 1. 编译并启动 Atria

```bash
go build -o atria ./cmd/atria
./atria serve
```

### 2. 初始化管理员

- 访问 http://127.0.0.1:8080/init
- 设置管理员用户名和密码
- 密码将使用 bcrypt 安全哈希存储

### 3. 登录后台

- 访问 http://127.0.0.1:8080/login
- 使用刚创建的管理员账号登录

### 4. 创建 API 凭据

- 访问 http://127.0.0.1:8080/credentials/new
- 填入从 https://my.telegram.org 获取的 API ID 和 API Hash
- 点击"创建凭据"

### 5. 选择 API 凭据

- 在顶部栏的下拉框中选择刚创建的 API 凭据

### 安全提醒

- 不要把 `data/` 目录提交到代码仓库
- 不要把真实 api_hash 提交到代码仓库
- 不要把真实手机号提交到代码仓库

## B. 登录验证

### 1. 开始登录

- 访问 http://127.0.0.1:8080/accounts/login
- 输入手机号（国际格式，如 +8613800138000）
- 点击"开始登录"

### 2. 输入验证码

- 检查 Telegram 应用或短信中的验证码
- 在验证码输入页面填写验证码
- 点击"提交验证码"

### 3. 处理两步验证（如果启用）

- 如果账号启用了两步验证，输入 2FA 密码
- 点击"提交密码"

### 4. 验证登录成功

- 登录成功后应跳转到账号详情页
- 检查显示：User ID、Username、显示名、状态：活跃、Session 状态：有效

### 5. 安全验证

- 检查 `data/sessions/` 目录，Session 文件应为加密内容
- 文件名格式：`session_{account_id}.enc`，不包含手机号
- 检查数据库 `telegram_accounts.phone_encrypted` 不是明文手机号
- 检查 `account_sessions` 不包含 Session 明文内容
- 检查控制台日志不包含：api_hash、手机号、验证码、2FA 密码、Session 内容

## C. 资料同步验证

### 1. 同步资料

- 在账号详情页点击"刷新资料"

### 2. 验证更新

- 检查 username、first_name、last_name 是否更新
- 检查 last_sync_at 时间是否更新

### 3. 审计日志

- 检查 `audit_logs` 表有 `account.profile_synced` 记录
- `metadata_json` 不应包含明文手机号

## D. Session 检测验证

### 1. 检测 Session

- 在账号详情页点击"检测 Session"

### 2. 验证状态

- 检查 Session 状态为 active
- 检查 last_verified_at 时间是否更新

### 3. 审计日志

- 检查 `audit_logs` 表有 `account.session_checked` 记录

## E. 本地删除 Session 验证

### 1. 本地删除 Session

- 在账号详情页点击"本地删除 Session"
- 确认删除

### 2. 验证结果

- 确认 Session 文件被删除（检查 `data/sessions/` 目录）
- 确认状态变为 deleted 或 logged_out
- 确认"刷新资料"按钮不再显示（或显示为不可用）
- 确认 Telegram 远端 Session 不一定被注销

### 3. 审计日志

- 检查 `audit_logs` 表有 `account.local_session_deleted` 记录

## F. 远端 Logout 验证

### 1. 重新登录账号

- 使用相同流程重新登录账号

### 2. 远端 Logout

- 在账号详情页点击"远端 Logout"
- 确认操作

### 3. 验证结果

- 确认本地 Session 文件被删除
- 确认状态变为 logged_out
- 确认需要重新登录才能继续管理

### 4. 审计日志

- 检查 `audit_logs` 表有 `account.remote_logout` 记录

## G. 异常场景验证

### 1. Session 文件缺失

- 登录账号后，手动移动或删除 Session 文件
- 点击"检测 Session"
- 应显示 Session 异常或失效提示

### 2. secret.key 更换

- 停止 Atria
- 删除 `data/secret.key`
- 重新启动 Atria（会生成新密钥）
- 尝试检测 Session
- 应显示 Session 解密失败提示

### 3. API 凭据禁用

- 禁用绑定的 API 凭据
- 尝试同步资料
- 应显示凭据已禁用提示

### 4. FLOOD_WAIT

- 如果遇到 FLOOD_WAIT
- 确认页面显示等待时间
- 确认不会自动重试

## H. 安全检查

### 1. 日志检查

- 搜索控制台日志
- 确认没有 api_hash 明文
- 确认没有验证码
- 确认没有 2FA 密码
- 确认没有 Session 内容
- 确认没有手机号明文

### 2. 数据库检查

- 使用 SQLite 浏览器打开 `data/atria.db`
- 检查 `telegram_accounts.phone_encrypted` 不是明文手机号
- 检查 `account_sessions` 不包含 Session 明文
- 检查 `audit_logs.metadata_json` 不包含敏感字段

### 3. 测试检查

- 确认 `go test ./...` 通过
- 确认普通测试不访问真实 Telegram 网络

## 已知限制

1. 真实登录需要有效的 Telegram API 凭据
2. FLOOD_WAIT 是 Telegram 的正常限制，不是 Atria 的问题
3. 首次登录可能需要等待验证码发送
4. 两步验证使用 SRP 协议，需要 gotd/td 库支持
5. secret.key 丢失会导致旧 Session 无法恢复
6. 本阶段不支持群组/频道同步
7. 本阶段不支持消息发送
8. 本阶段不支持批量操作
