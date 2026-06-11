# Vue SPA 迁移文档

> 聊天底层协议实现已通过 `telegramclient.ClientAdapter` 隔离，当前实现是 gotd，未来可替换 TDLib。详见 `docs/21_telegram_client_adapter.md`。

## 迁移目标

将 Atria 登录后的所有页面从 Go Template 迁移为 Vue SPA 架构，同时完整保留现有 UI 风格。

## 旧模板备份

- `web/templates_legacy/` — 旧 Go Template 备份
- `web/static_legacy/` — 旧 CSS/JS 备份

## 路由映射

### Canonical URL 规则

所有 SPA 页面 URL 统一为 `/app/#/...` 格式（hash router）：

| 页面 | Canonical URL |
|------|---------------|
| Dashboard | `/app/#/dashboard` |
| 账号列表 | `/app/#/accounts` |
| 账号接入 | `/app/#/accounts/login` |
| 账号详情 | `/app/#/accounts/:id` |
| 聊天 | `/app/#/chats` |
| 聊天详情 | `/app/#/chats/:peerRef` |
| 联系人 | `/app/#/contacts` |
| 审计日志 | `/app/#/audit` |
| 系统设置 | `/app/#/settings` |

**禁止出现**：`/app/accounts#/dashboard`、`/app/chats#/chats`、`/app/app/...`

### 旧路由重定向表

| 旧路由 | 重定向目标 | 说明 |
|--------|------------|------|
| `/` | 无变化 | 保留旧仪表盘（Go Template） |
| `/accounts` | `/app/#/accounts` | 302 重定向 |
| `/accounts/login` | `/app/#/accounts/login` | 302 重定向 |
| `/accounts/:id` | `/app/#/accounts/:id` | 302 重定向 |
| `/chats` | `/app/#/chats` | 302 重定向 |
| `/chats/:peer_ref` | `/app/#/chats/:peerRef` | 302 重定向 |
| `/contacts` | `/app/#/contacts` | 302 重定向 |
| `/audit` | `/app/#/audit` | 302 重定向 |
| `/settings` | `/app/#/settings` | 302 重定向 |
| `/security` | `/app/#/settings` | 302 重定向 |
| `/app/*` | `/app/#/*` | Go 后端重定向到 canonical hash URL |
| `/app` | SPA shell | 返回 index.html |
| `/app/` | SPA shell | 返回 index.html |
| `/login` | 无变化 | 保留 Go Template |
| `/init` | 无变化 | 保留 Go Template |

### 为什么使用 /app/#/...

1. Go 后端只需服务 `/app` 和 `/app/` 返回 SPA shell
2. `/app/*` 的其他路径全部 302 重定向到 `/app/#/*`
3. 浏览器不发送 hash 部分给后端，所以后端不会误处理 hash 路由
4. 前端 `main.ts` 有 canonicalization 兜底，防止 history-style URL 直接加载
5. 所有 Sidebar/Topbar 导航只产生 `/app/#/...` 格式

### 已迁移页面

1. **Dashboard** — `/app/#/dashboard` — 统计卡片、安全提示、快速开始、系统信息
2. **Accounts** — `/app/#/accounts` — 账号列表、Session 状态
3. **Account Detail** — `/app/#/accounts/:id` — 账号信息、Session 信息
4. **Account Login** — `/app/#/accounts/login` — 手机号→OTP→2FA 全异步流程
5. **Chats** — `/app/#/chats` — 会话列表、消息历史、发送消息
6. **Contacts** — `/app/#/contacts` — 开发中占位
7. **Audit** — `/app/#/audit` — 审计日志列表
8. **Settings** — `/app/#/settings` — 管理员安全、API Key、代理、系统信息

## 样式保持原则

- 所有 CSS 变量来自旧版 `app.css`
- sidebar 宽度 240px
- topbar 高度 56px
- 深色/浅色主题变量完全一致
- 卡片、按钮、输入框、表格、badge、alert 样式保持一致
- 品牌渐变色保持一致

## JSON API

| API | 说明 |
|-----|------|
| `GET /api/me` | 当前管理员和账号信息 |
| `GET /api/dashboard/stats` | 仪表盘统计 |
| `GET /api/accounts` | 账号列表 |
| `GET /api/accounts/:id` | 账号详情 |
| `GET /api/audit` | 审计日志 |
| `GET /api/settings` | 系统设置 |
| `GET /api/chats/dialogs` | 聊天会话列表 |
| `GET /api/chats/:peer_ref/messages` | 消息历史 |
| `POST /api/chats/:peer_ref/messages` | 发送消息 |

## 构建

```bash
cd frontend
npm install
npm run build
```

构建产物输出到 `web/static/dist/`，Go embed 自动包含。

## 回退方式

如需回退到旧模板：
1. 将 `web/templates_legacy/` 复制回 `web/templates/`
2. 修改 `internal/server/router.go` 中的路由重定向
3. 重新构建

## 真实验收回归修复

### 聊天页 full-width 布局规则

- 聊天页 (`/app/chats`) 使用 `page-full` class，移除 padding，禁止页面级滚动
- 普通页面（Dashboard、Settings、Accounts）保持 `padding: 24px` 和页面级滚动
- `AppShell` 根据 `route.path` 自动判断是否启用 full-width 模式
- `.app-layout` 使用 `height: 100vh` 替代 `min-height: 100vh`，确保 flex 布局正确撑满
- `.chat-layout` 使用 `height: 100%; width: 100%`，不受 max-width 限制

### 普通页面和聊天页布局差异

| 属性 | 普通页面 | 聊天页 |
|------|----------|--------|
| padding | 24px | 0 |
| overflow | auto (页面滚动) | hidden (内部滚动) |
| width | 受 max-width 限制 | 100% 填满 |
| height | 内容撑开 | 100vh - topbar |

### Settings API Key 编辑态规则

- 展示态：显示名称、脱敏 API ID、API Hash hint、状态
- 点击"修改配置"：切换到编辑态（`apiKeyEditMode = true`）
- 编辑态：显示名称、API ID、API Hash 输入框
- API Hash placeholder："留空则保持原值"
- 保存成功：退出编辑态，刷新数据
- 取消：退出编辑态，不保存
- loading 时按钮 disabled

### Settings 代理配置兼容旧 system_settings 规则

- `/api/settings` 读取 `system_settings` 表中的 `proxy_*` 字段
- `proxy_enabled=true` 时显示代理类型、host、port 等
- `proxy_enabled` 缺失或 `proxy_type=none` 时显示"不使用代理"
- `proxy_password` 不回显明文
- `proxy_password` 留空时保持旧值
- 保存写回同一套 `system_settings` key
- 页面加载不会覆盖旧代理配置（watcher 从 API 响应初始化表单）

## Vue Router Hash Mode

- 使用 `createWebHashHistory('/app/')` 替代 `createWebHistory('/app/')`
- 最终 URL 格式：`/app/#/dashboard`、`/app/#/chats/u_123`
- 优势：降低 Go 后端 fallback 复杂度，避免刷新/复制链接/旧路由跳转问题
- 旧 history URL（如 `/app/chats/u_123`）通过 Go 后端 302 重定向到 `/app/#/chats/u_123`

## Query Key 规则

### dialogs query key

```
['dialogs', accountId]
```

必须包含 `accountId`，确保切换账号时缓存隔离。

### messages query key

```
['messages', accountId, peerRef]
```

必须包含 `accountId` 和 `peerRef`，确保：
- 不同账号的同一 peerRef 不共享缓存
- 不同 peer 的消息不串数据

### 切换账号处理

- 切换账号时清空 `selectedPeerRef`
- 跳回 `/app/#/chats`
- 清理或 scope query cache
- 不显示旧账号消息

### 直接访问 peer URL

- 从 route.params.peerRef 初始化 selectedPeerRef
- 加载 dialogs（含 accountId）
- 加载 messages（含 accountId + peerRef）
- 失败时显示明确错误，不空白

## 聊天缓存策略

### Cache-first 加载

- 会话列表优先从 `chat_peer_cache` 表读取，再后台刷新 Telegram
- 消息历史优先从 `chat_message_cache` 表读取，再后台刷新 Telegram
- Telegram 刷新失败时保留缓存数据，返回 `stale: true`
- 前端 TanStack Query 使用 `staleTime: 30s` 避免重复请求

### 消息缓存（ChatMessageCache）

- 按 `account_id + peer_ref + telegram_message_id` 唯一
- 消息正文使用 AES-256-GCM 加密存储
- 每个 peer 最多缓存 100 条最近消息
- 不做全量历史扫描
- 不做自动后台同步

### 参考 Telegram Web 但不复制源码

- 参考产品体验：先显示本地数据，再后台刷新
- 参考缓存架构：按账号/会话隔离，限制缓存大小
- 不复制 Telegram Web 源码、CSS、图标、品牌资产
- 不引入 Telegram Web 的 GPL 源码

## 消息排版修复

- 每条消息独立 block，不使用 absolute 定位
- 长文本自动换行：`overflow-wrap: anywhere; word-break: break-word`
- 时间显示在气泡底部，不覆盖正文
- 关闭 MessageList 虚拟滚动（动态高度导致重叠）
- DialogList 保留虚拟滚动

## 安全约束

- 不记录 api_hash、proxy_password、session path、access_hash
- 不记录完整消息正文
- 不记录 OTP、2FA 密码
- 用户消息使用 escapeHtml，不使用 v-html
- access_hash 加密存储在 chat_peer_cache
- 消息正文加密存储在 chat_message_cache
