# Vue SPA 迁移文档

## 迁移目标

将 Atria 登录后的所有页面从 Go Template 迁移为 Vue SPA 架构，同时完整保留现有 UI 风格。

## 旧模板备份

- `web/templates_legacy/` — 旧 Go Template 备份
- `web/static_legacy/` — 旧 CSS/JS 备份

## 路由映射

| 旧路由 | 新路由 | 状态 |
|--------|--------|------|
| `/` | `/app/dashboard` | 旧路由保留兼容 |
| `/accounts` | `/app/accounts` | 旧路由保留兼容 |
| `/accounts/login` | `/app/accounts/login` | 旧路由保留兼容 |
| `/accounts/:id` | `/app/accounts/:id` | 旧路由保留兼容 |
| `/chats` | `/app/chats` | 重定向到 SPA |
| `/chats/:peer_ref` | `/app/chats/:peerRef` | 重定向到 SPA |
| `/contacts` | `/app/contacts` | 旧路由保留兼容 |
| `/audit` | `/app/audit` | 旧路由保留兼容 |
| `/settings` | `/app/settings` | 旧路由保留兼容 |
| `/login` | 无变化 | 保留 Go Template |
| `/init` | 无变化 | 保留 Go Template |

## 已迁移页面

1. **Dashboard** — `/app/dashboard` — 统计卡片、安全提示、快速开始、系统信息
2. **Accounts** — `/app/accounts` — 账号列表、Session 状态
3. **Account Detail** — `/app/accounts/:id` — 账号信息、Session 信息
4. **Account Login** — `/app/accounts/login` — 手机号→OTP→2FA 全异步流程
5. **Chats** — `/app/chats` — 会话列表、消息历史、发送消息
6. **Contacts** — `/app/contacts` — 开发中占位
7. **Audit** — `/app/audit` — 审计日志列表
8. **Settings** — `/app/settings` — 管理员安全、API Key、代理、系统信息

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

## 安全约束

- 不记录 api_hash、proxy_password、session path、access_hash
- 不记录完整消息正文
- 不记录 OTP、2FA 密码
- 用户消息使用 escapeHtml，不使用 v-html
