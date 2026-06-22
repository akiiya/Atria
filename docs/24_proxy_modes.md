# 代理模式说明

## 支持的代理类型

| 类型 | 值 | 适用协议 | 说明 |
|------|-----|----------|------|
| 不使用代理 | `none` | - | 直连 Telegram |
| HTTPS 代理 | `https` | MTProto | HTTP CONNECT 隧道，建立到目标地址的 raw TCP 连接 |
| SOCKS5 代理 | `socks5` | MTProto | SOCKS5 协议代理 |
| API Proxy | `api_proxy` | HTTP API（仅限） | HTTPS API endpoint override，不适用于 MTProto |

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

### api_proxy（API Proxy）

HTTPS API endpoint override。用于将 Telegram HTTP API 请求路由到自定义域名。

**重要限制：**
- 仅适用于 Telegram HTTP API（如 Bot API `https://api.telegram.org`）
- **不适用于** MTProto 协议
- 当前 Atria 全部使用 MTProto（gotd/td），不使用 HTTP API
- 选择此模式后，MTProto 链路（登录、聊天、runtime）将返回明确错误

**Cloudflare Worker 反代场景：**

用户可以通过 Cloudflare Worker 反代 Telegram HTTP API：
```
用户 -> https://xxx.domain.com -> Cloudflare Worker -> https://api.telegram.org
```

但这种反代方式**无法承载 MTProto TCP 连接**，因为：
1. MTProto 是二进制协议，不是 HTTP
2. Cloudflare Worker 只能处理 HTTP/HTTPS 请求
3. MTProto 需要持久 TCP 连接，不是请求-响应模式

## 如何选择代理类型

| 场景 | 推荐代理类型 |
|------|-------------|
| 服务器可直接访问 Telegram | none |
| 需要通过 HTTP 代理访问 | https |
| 需要通过 SOCKS5 代理访问 | socks5 |
| 有 Cloudflare Worker 反代 HTTP API | api_proxy（仅保存配置，不影响 MTProto） |
| 需要通过代理访问 MTProto | socks5 或 https |

## API Proxy 不适用于 MTProto 的原因

gotd/td 库使用 MTProto 协议与 Telegram 通信：

1. **协议不同**：MTProto 是二进制协议，HTTP API 是 JSON over HTTPS
2. **连接方式不同**：MTProto 需要持久 TCP 连接，HTTP API 是请求-响应
3. **地址不同**：MTProto 连接 Telegram DC 地址（如 `149.154.167.50:443`），HTTP API 连接 `api.telegram.org`
4. **认证方式不同**：MTProto 使用 MTProto 密钥交换，HTTP API 使用 Bot Token

因此，`https://xxx.domain.com` 这样的 API Proxy URL 无法用于 gotd/MTProto 连接。

## Runtime 代理注入

### 已修复的 Bug

在之前的版本中，RuntimeManagerImpl 未注入 proxy dialer，导致 runtime 操作（updates、executor 路径）绕过代理。

修复后：
- Server 启动时从数据库读取代理配置
- 将 proxy dialer 注入 RuntimeManagerImpl
- Runtime 的 telegram.Client 使用注入的 dialer

### api_proxy 对 Runtime 的影响

如果选择 api_proxy 模式：
- `BuildProxyDialerFromDB()` 返回明确错误
- Runtime dialer 注入失败，日志警告"将使用直连"
- Runtime 使用直连而不是 api_proxy URL
- 不会导致 infinite connecting

## 代理配置存储

代理配置存储在 `system_settings` 表中：

| Key | 说明 |
|-----|------|
| `proxy_enabled` | 是否启用代理（"true"/"false"） |
| `proxy_type` | 代理类型（"none"/"https"/"socks5"/"api_proxy"） |
| `proxy_host` | 代理主机（socks5/https 使用） |
| `proxy_port` | 代理端口（socks5/https 使用） |
| `proxy_username` | 代理用户名（可选） |
| `proxy_password` | 代理密码（AES-256-GCM 加密） |
| `proxy_timeout` | 超时秒数 |
| `proxy_remark` | 备注 |
| `api_proxy_url` | API Proxy URL（api_proxy 使用） |

## 安全说明

- `proxy_password` 使用 AES-256-GCM 加密存储
- API 返回设置时不包含 `proxy_password`
- 日志不记录完整 proxy_password
- API Proxy URL 中的敏感 path/token 不记录到日志
