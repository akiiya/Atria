# Vue SPA 迁移文档

## 迁移目标

将 Atria 登录后的所有页面从 Go Template 迁移为 Vue SPA 架构，同时完整保留现有 UI 风格。

## 旧模板备份

- `web/templates_legacy/` — 旧 Go Template 备份
- `web/static_legacy/` — 旧 CSS/JS 备份

## 路由映射

| 旧路由 | 新路由 | 状态 |
|--------|--------|------|
| `/` | 无变化 | 保留旧仪表盘（Go Template） |
| `/accounts` | `/app/accounts` | 重定向到 SPA |
| `/accounts/login` | `/app/accounts/login` | 重定向到 SPA |
| `/accounts/:id` | `/app/accounts/:id` | 重定向到 SPA |
| `/chats` | `/app/chats` | 重定向到 SPA |
| `/chats/:peer_ref` | `/app/chats/:peerRef` | 重定向到 SPA |
| `/contacts` | `/app/contacts` | 重定向到 SPA |
| `/audit` | `/app/audit` | 重定向到 SPA |
| `/settings` | `/app/settings` | 重定向到 SPA |
| `/security` | `/app/settings` | 重定向到 SPA |
| `/login` | 无变化 | 保留 Go Template |
| `/init` | 无变化 | 保留 Go Template |

**注意**：`/` 保留旧仪表盘是为了兼容现有测试和用户习惯。如需切换到 SPA，将 `/` 重定向到 `/app/dashboard`。

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
- 旧 history URL（如 `/app/chats/u_123`）通过 Go 后端 SPA handler 兼容

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
