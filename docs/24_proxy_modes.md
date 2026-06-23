# 代理模式说明

## 支持的代理类型

| 类型 | 值 | 适用协议 | 说明 |
|------|-----|----------|------|
| 不使用代理 | `none` | - | 直连 Telegram |
| HTTPS 代理 | `https` | MTProto | HTTP CONNECT 隧道，建立到目标地址的 raw TCP 连接 |
| SOCKS5 代理 | `socks5` | MTProto | SOCKS5 协议代理 |

## 各代理类型详细说明

### none（不使用代理）

直接连接 Telegram DC 地址（如 `149.154.167.50:443`）。

适用于：
- 服务器可以直接访问 Telegram
- 不需要绕过网络限制

### https（HTTPS 代理）

使用 HTTP CONNECT 隧道协议。客户端连接到代理服务器，发送 CONNECT 请求建立到目标地址的 TCP 隧道。

工作流程：
1. 连接到代理服务器（如 `proxy.example.com:443`）
2. 发送 `CONNECT 149.154.167.50:443 HTTP/1.1`
3. 代理返回 `200 Connection Established`
4. 通过隧道进行 MTProto 通信

适用于：
- 需要通过 HTTP 代理访问 Telegram
- 企业网络环境

### socks5（SOCKS5 代理）

使用 SOCKS5 协议。客户端通过 SOCKS5 代理建立到目标地址的 TCP 连接。

适用于：
- 需要通过 SOCKS5 代理访问 Telegram
- 常见的代理软件（如 Shadowsocks、V2Ray 等）

## 如何选择代理类型

| 场景 | 推荐代理类型 |
|------|-------------|
| 服务器可直接访问 Telegram | none |
| 需要通过 HTTP 代理访问 | https |
| 需要通过 SOCKS5 代理访问 | socks5 |
| 需要通过代理访问 MTProto | socks5 或 https |

## 关于 API Proxy（已移除）

曾经评估过 API Proxy 模式，允许用户填写 HTTPS URL（如 `https://xxx.domain.com`）作为 Telegram HTTP API endpoint override。

**已移除原因：**

当前 Atria 的登录、聊天、runtime、updates 全部基于 gotd/MTProto 协议，不存在 Telegram HTTP API client。

MTProto 与 HTTP API 的差异：
1. **协议不同**：MTProto 是二进制协议，HTTP API 是 JSON over HTTPS
2. **连接方式不同**：MTProto 需要持久 TCP 连接，HTTP API 是请求-响应
3. **地址不同**：MTProto 连接 Telegram DC 地址（如 `149.154.167.50:443`），HTTP API 连接 `api.telegram.org`

Cloudflare Worker HTTPS 反代 Telegram HTTP API 无法承载 MTProto TCP 连接，因此 API Proxy 已从配置选项中移除。

**Legacy 处理：**

如果旧数据库中已保存 `proxy_type=api_proxy`：
- 系统不会崩溃
- 不会静默转为直连
- Runtime status 显示 `proxy_config_invalid`
- Settings 页面提示当前代理配置已失效
- 用户需要重新选择 SOCKS5 / HTTPS CONNECT / none

## Runtime 代理注入

### 已修复的 Bug

在之前的版本中，RuntimeManagerImpl 未注入 proxy dialer，导致 runtime 操作（updates、executor 路径）绕过代理。

修复后：
- Server 启动时从数据库读取代理配置
- 将 proxy dialer 注入 RuntimeManagerImpl
- Runtime 的 telegram.Client 使用注入的 dialer

## 代理配置存储

代理配置存储在 `system_settings` 表中：

| Key | 说明 |
|-----|------|
| `proxy_enabled` | 是否启用代理（"true"/"false"） |
| `proxy_type` | 代理类型（"none"/"https"/"socks5"） |
| `proxy_host` | 代理主机（socks5/https 使用） |
| `proxy_port` | 代理端口（socks5/https 使用） |
| `proxy_username` | 代理用户名（可选） |
| `proxy_password` | 代理密码（AES-256-GCM 加密） |
| `proxy_timeout` | 超时秒数 |
| `proxy_remark` | 备注 |

## 代理设置热生效

### 行为

代理配置保存后，**运行时立即生效**，不需要重启服务。

保存流程：
1. 前端调用 `POST /api/settings/proxy`
2. 后端持久化配置到数据库
3. 后端调用 `RuntimeManager.OnProxySettingsChanged()`
4. RuntimeManager 重建 dialer（从数据库重新读取配置）
5. RuntimeManager 停止所有运行时（它们会用旧 dialer）
6. 前端刷新 runtime status
7. 用户可手动重新启动 runtime，或等待自动启动

### 保存 SOCKS5 / HTTPS CONNECT

1. 页面保存成功
2. 后端立即持久化配置
3. 后端重建 dialer 并停止所有运行时
4. REST temporary client 立即使用新配置（每次请求都从 DB 读取）
5. Runtime 下次 start 时使用新 dialer
6. 前端 runtime status 刷新

### 保存 none / direct

1. 页面保存成功
2. 后端重建 dialer（直连）
3. 后端停止所有运行时
4. 后续连接使用直连

### 不清空的内容

代理配置变更后：
- 不清空聊天消息缓存
- 不删除 session
- 不删除 account
- 不清空 dialogs/messages query cache
- 只重置 runtime/network connection 状态

## 安全说明

- `proxy_password` 使用 AES-256-GCM 加密存储
- API 返回设置时不包含 `proxy_password`
- 日志不记录完整 proxy_password
