# Telegram API Key 与 Session 说明

## 核心概念

### API Key（API 凭据）

API Key 是 Telegram API 应用凭据，由 `api_id` 和 `api_hash` 组成。

- `api_id`：整数类型的应用标识
- `api_hash`：32 位十六进制字符串的应用密钥

**关键理解：**

- API Key 是**应用级凭据**，不是用户账号
- 通常一个 Atria 实例只需要**一套默认 API Key**
- 多个 Telegram 账号可以**共用同一套 API Key** 登录
- 不需要为每个 Telegram 账号单独申请 API Key

### Telegram 账号

Telegram 账号是通过以下方式登录的用户账号：

1. 手机号
2. 验证码（通过 Telegram 发送）
3. 两步验证密码（可选，如果启用了 2FA）

登录成功后，系统会生成一个独立的 **Session**。

### Session

Session 是某个 Telegram 账号登录成功后的 MTProto 会话。

- 每个 Session 绑定登录时使用的 `credential_id`（API Key）
- Session 文件加密保存在本地
- 后续操作（聊天、状态检测、资料同步、注销）使用 Session 绑定的 `credential_id`
- 修改系统默认 API Key **不会**自动注销已有 Session
- 已有 Session 继续使用登录时绑定的 API Key

## 获取 API Key

### 申请步骤

1. 前往 [my.telegram.org](https://my.telegram.org)
2. 使用你的 Telegram 账号手机号登录
3. 进入 **API development tools**
4. 创建 application
5. 复制 `api_id` 和 `api_hash`

### 网络建议

申请 API Key 时，建议注意以下事项以提高成功率：

- 使用干净、稳定、低风险的网络环境
- 尽量避免使用机房 IP、公共代理、低评分 IP
- 网络出口地区建议与手机号常用地区保持一致

> **注意：** 以上属于提高成功率的经验建议，不是 Telegram 官方的硬性规则。

## 初始化配置

首次启动 Atria 时，初始化页面会引导你：

1. 设置管理员账号（用户名 + 密码）
2. 配置 Telegram API Key（可选，但推荐）

如果初始化时跳过了 API Key 配置，后续可以在 **系统设置 > Telegram API Key** 中添加。

## API Key 管理

### 默认 API Key

- 系统中的第一套 API Key 自动成为默认
- 只能有一个默认 API Key
- 默认 API Key 用于新账号登录
- 禁用默认 API Key 时，系统会自动切换到其他启用的 API Key
- 不能删除默认 API Key（需要先切换默认）

### 多 API Key

多套 API Key 属于高级用法，适用于：

- 备用 API Key
- 隔离不同用途
- 风险策略管理

## 账号管理

### 接入新账号

1. 确保已配置默认 API Key
2. 进入 **账号管理 > 接入账号**
3. 输入 Telegram 手机号
4. 输入收到的验证码
5. 如果启用了两步验证，输入 2FA 密码
6. 登录成功后，Session 自动保存

### Session 生命周期

- **active**：Session 有效，可正常使用
- **expired**：Session 过期，需要重新登录
- **invalid**：Session 失效（可能被 Telegram 踢出）

## API 网络代理

在中国大陆等无法直连 Telegram 的环境，需要配置代理。

### 支持的代理类型

- **不使用代理**：直连 Telegram（适用于可直连的网络环境）
- **HTTPS 代理**：通过 HTTP CONNECT 代理连接
- **SOCKS5 代理**：通过 SOCKS5 代理连接

### 配置方式

进入 **系统设置 > API 网络代理**，填写：

- 代理类型
- 主机地址
- 端口
- 用户名（可选）
- 密码（可选，加密保存）

> **注意：** 代理配置仅用于 Telegram MTProto API 连接，不影响 Atria Web 界面的访问。

## 安全说明

### 加密存储

以下敏感数据使用 AES-256-GCM 加密存储：

- `api_hash`
- `proxy password`
- Telegram Session 文件
- 手机号

### 密钥文件

Atria 使用本地 master key（`data/secret.key`）加密敏感数据。

**重要：** 请务必备份 `data/secret.key` 文件。丢失此文件后：

- 无法恢复已保存的 API Key
- 无法恢复已保存的 Session
- 无法解密代理密码

### 不支持的功能

Atria 不支持以下功能，也不会在未来实现：

- 批量登录
- 接码平台
- 批量私信
- 群发消息
- 自动拉群
- 刷量/采集
- 规避 Telegram 风控
