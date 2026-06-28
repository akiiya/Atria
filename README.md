# Atria

> A lightweight self-hosted MTProto session manager built with Go.
>
> 使用 Go 构建的轻量级自托管 MTProto Session 管理器。

**Atria** 是一个轻量、自托管的 MTProto 多账号 Session 管理面板。它通过安全的内嵌 Web 界面，管理多个 Telegram 兼容的 MTProto 用户 Session、API 凭据配置、账号元信息和审计日志。

> **当前版本：v0.1.0-rc.1** — Release Candidate，已完成人工验收，支持聊天、联系人、搜索、媒体、群组/频道、国际化、审计、维护等核心功能。

> **Atria 与 Telegram 无关（Atria is not affiliated with Telegram）。**
>
> **Atria 不是垃圾消息、刷量、绕过平台限制或批量骚扰工具。**
>
> **Atria 不实现批量消息、批量邀请或平台限制绕过功能。**

---

## 核心特性

- **多账号 Session 管理** — 从单一面板管理多个 Telegram 兼容 MTProto 用户 Session
- **真实 MTProto 登录** — 支持手机号验证码登录、两步验证（2FA）、加密 Session 保存
- **账号资料同步** — 基于已保存 Session 同步 Telegram 账号基础资料
- **Session 状态检测** — 检测已保存 Session 的有效性
- **远端 Logout** — 通过 MTProto 注销 Telegram 远端 Session，同时删除本地文件
- **本地删除 Session** — 仅删除本地加密 Session 文件，不调用 Telegram
- **Session 生命周期管理** — 完整处理登录→保存→同步→检测→失效→重新登录闭环
- **API 凭据管理** — 配置和切换多组 API ID/Hash，支持独立的风险策略
- **加密存储** — Session 文件加密存储，数据库中不保存明文 Session 数据
- **审计日志** — 追踪所有管理操作的详细审计记录，自动脱敏敏感字段
- **聊天浏览** — 实时消息同步、历史加载、发送文本消息、new/edit/delete 实时推送
- **联系人** — 联系人列表、搜索、cache-first、点击跳转聊天
- **搜索** — 本地消息缓存全文搜索，支持 peer_ref 限定
- **媒体** — 图片/文档/视频/音频识别、下载、预览、缓存管理
- **群组与频道** — user/bot/chat/supergroup/channel 类型识别与展示
- **数据维护** — 表统计、缓存统计、dry-run 清理、媒体缓存管理
- **仪表盘** — 统计卡片、最近审计事件、快捷入口
- **国际化** — 10 种语言（341 key/locale）、浏览器语言检测、localStorage 持久化
- **Web 自更新** — 设置页可检查更新、下载、Dry-Run 验证、应用更新（alpha 能力）
- **现代化 Web 界面** — Vue 3 SPA 全屏管理面板，支持浅色/深色/跟随系统主题
- **单二进制部署** — 构建产物为单一可执行文件，所有 Web 资源内嵌，运行时无外部依赖
- **SQLite 默认** — 零配置即可启动；可选 PostgreSQL / MySQL / MariaDB

## 远端 Logout 与本地删除 Session

Atria 提供两种移除账号 Session 的方式：

**远端 Logout：**
- 通过 MTProto 调用 Telegram 的 auth.logOut
- 使该 Session 在 Telegram 远端失效
- 同时删除本地加密 Session 文件
- 删除后需要重新登录才能继续管理该账号

**本地删除 Session：**
- 仅删除 Atria 本地保存的加密 Session 文件
- 不调用 Telegram 远端注销
- Telegram 远端 Session 可能仍然有效
- 删除后需要重新登录才能继续管理该账号

## 非目标

Atria 专为合法的 Session 管理设计，**明确不支持**：

- 批量消息或垃圾信息
- 批量邀请成员或加入群组
- 绕过平台限制
- 账号交易或账号市场
- 手机号采集或接码平台
- 批量登录或批量 Logout
- 任何形式的自动化骚扰

## 快速开始

```bash
# 构建
go build -o atria ./cmd/atria

# 运行（默认监听 http://127.0.0.1:8080）
./atria serve
```

首次启动后，访问 Web 界面初始化管理员账号。

## 安装

### Linux 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/akiiya/Atria/main/scripts/install.sh | bash
```

安装后访问 http://127.0.0.1:8080

**Try-Run 模式（不写入系统路径）：**
```bash
ATRIA_INSTALL_DRY_RUN=1 bash install.sh
```

### 手动安装

从 [GitHub Releases](https://github.com/akiiya/Atria/releases) 下载对应平台的压缩包，解压后运行 `./atria serve`。

### 从源码构建

```bash
go build -o atria ./cmd/atria
./atria serve
```

## 配置

Atria 支持零配置启动，所有设置都有合理默认值：

| 配置项 | 默认值 | 环境变量 |
|--------|--------|----------|
| 监听地址 | 127.0.0.1 | ATRIA_HOST |
| 监听端口 | 8080 | ATRIA_PORT |
| 数据目录 | ./data | ATRIA_DATA_DIR |
| 数据库驱动 | sqlite | ATRIA_DB_DRIVER |
| 数据库连接 | ./data/atria.db | ATRIA_DB_DSN |
| Session 目录 | ./data/sessions | ATRIA_SESSION_DIR |
| 日志目录 | ./data/logs | ATRIA_LOG_DIR |
| 加密密钥 | （自动生成） | ATRIA_SECRET_KEY |

## 数据目录

Atria 所有运行时数据存储在 `./data/` 目录下：

```
data/
├── atria.db          # SQLite 数据库
├── secret.key        # 加密密钥（自动生成，权限 0600）
├── sessions/         # 加密的 MTProto Session 文件
└── logs/             # 应用日志
```

**重要提示：** 请务必备份 `secret.key` 文件。丢失密钥将导致加密的 Session 数据无法恢复。

## 安全

- 密码使用 bcrypt 哈希存储（永不保存明文）
- API Hash、手机号在数据库中 AES-256-GCM 加密存储
- Session 文件使用本地密钥加密
- 审计日志自动脱敏 17 类敏感字段（password、api_hash、session、token、code、secret、access_hash、file_reference、local_path、message_body、search_keyword 等）
- 所有状态变更操作提供 CSRF 保护（double-submit cookie）
- Cookie 安全属性：HttpOnly、SameSite（Lax/Strict/None 可配置）
- Cookie Secure 默认关闭（本地开发），**生产环境必须设置 `ATRIA_COOKIE_SECURE=true`**
- WebSocket 连接强制同源检查
- 搜索结果 HTML 转义防 XSS
- 错误页面 HTML 实体转义防 XSS
- API 响应中的错误消息通过 `SanitizeErrorMessage` 脱敏
- 媒体内容端点设置 CSP 和 X-Content-Type-Options 安全头
- 路径穿越防护（媒体文件访问）

详见 [docs/06_security_policy.md](docs/06_security_policy.md)。

## 风险策略

每个 API 凭据可配置独立的风险策略：

- **disabled** — 禁止高风险操作（默认）
- **enabled** — 允许高风险操作
- **confirm** — 需要用户明确确认

## 部署理念

- **单二进制** — 构建产物为单一可执行文件
- **默认 SQLite** — 零配置即可启动
- **无配置启动** — 所有设置有合理默认值
- **可选数据库** — 支持 PostgreSQL / MySQL / MariaDB

## Web 自更新

设置页支持检查更新、下载、Dry-Run 验证和应用更新。

- 真实 GitHub Release 自更新需要 tag 和 release 产物
- Docker 环境不支持容器内自更新，请使用新镜像重建
- 自更新仍为 alpha 能力，详见 [docs/12_web_self_update.md](docs/12_web_self_update.md)
- 普通用户可继续使用 `install.sh` 手动升级

## 发布流程

发布版本以根目录 `VERSION` 文件为唯一来源。

**分支语义：**
- `dev` — 验证流程（测试、构建、VERSION 格式检查），不发布
- `main` — 发布流程（读取 VERSION → 自动创建 tag → 构建 → GitHub Release）

**发布步骤：**
1. 在 dev 分支修改 VERSION 文件（如 `v0.1.0`、`v0.2.0-rc.1`）
2. 创建 PR（dev → main），CI 验证通过后合并
3. main push 触发 release workflow：
   - 读取 VERSION → 校验 SemVer → 检查 tag 是否已存在
   - tag 不存在则自动创建 → 构建多平台产物 → 创建 GitHub Release
   - VERSION 含 `-rc`/`-alpha`/`-beta` 时标记为 prerelease
4. 如果 VERSION 未升级且 tag 已存在，workflow 失败

**本地构建：**
```bash
VERSION=$(cat VERSION) bash scripts/build_release.sh
```

不再需要手动 `git tag` 或手动创建 GitHub Release。

## 技术栈

- **Go** + **Gin** Web 框架
- **GORM** ORM，默认 SQLite，可选 PostgreSQL / MySQL / MariaDB
- **Go embed** 实现单二进制 Web 资源嵌入
- **Vue 3** + **Pinia** + **TanStack Query** 前端 SPA
- **gotd/td** MTProto 客户端（通过中立 DTO 边界隔离）
- **nhooyr.io/websocket** WebSocket 实时推送

## 项目结构

```
atria/
├── VERSION                 # 发布版本号（唯一版本来源）
├── cmd/atria/              # 应用入口
├── frontend/               # Vue 3 SPA 前端
│   └── src/
│       ├── api/            # API 封装
│       ├── features/       # 页面组件（chat, contacts, search, audit...）
│       ├── i18n/           # 国际化（10 种语言）
│       ├── stores/         # Pinia 状态管理
│       └── types/          # TypeScript 类型定义
├── internal/
│   ├── account/            # 账号服务
│   ├── audit/              # 审计日志（自动脱敏）
│   ├── auth/               # 认证、CSRF、Session
│   ├── chat/               # 聊天服务（gotd 无关）
│   ├── config/             # 配置加载
│   ├── crypto/             # AES-256-GCM 加密
│   ├── database/           # 数据库初始化
│   ├── media/              # 媒体下载与缓存
│   ├── migration/          # 版本化数据迁移（当前 v13）
│   ├── model/              # GORM 数据模型
│   ├── mtproto/            # MTProto 客户端封装
│   ├── network/            # 代理支持
│   ├── security/           # 错误消息脱敏
│   ├── server/             # HTTP 服务器、路由、WebSocket
│   ├── telegramclient/     # Telegram 适配器（中立 DTO 边界）
│   ├── updater/            # 自更新
│   ├── version/            # 版本信息
│   └── web/                # 嵌入式 Web 资源访问
├── web/
│   ├── templates/          # HTML 模板
│   └── static/dist/        # Vue SPA 构建产物（Go embed）
└── docs/                   # 项目文档
```

## 文档

- [项目范围](docs/01_project_scope.md)
- [架构设计](docs/03_architecture.md)
- [数据模型](docs/04_data_model.md)
- [开发计划](docs/05_development_plan.md)
- [安全策略](docs/06_security_policy.md)
- [UI 设计指南](docs/07_ui_design_guidelines.md)
- [数据迁移](docs/17_data_migrations.md)
- [Telegram 客户端适配器](docs/21_telegram_client_adapter.md)
- [WebSocket 实时推送](docs/23_websocket_realtime_push.md)
- [联系人 MVP](docs/25_contacts_mvp.md)
- [审计日志 MVP](docs/26_audit_logs_mvp.md)
- [仪表盘 MVP](docs/27_dashboard_mvp.md)
- [国际化 MVP](docs/28_i18n_mvp.md)
- [数据维护 MVP](docs/29_maintenance_mvp.md)
- [账号会话 MVP](docs/30_accounts_sessions_mvp.md)
- [搜索 MVP](docs/31_search_mvp.md)
- [人工浏览器验收清单](docs/32_manual_browser_acceptance.md)
- [媒体 MVP](docs/33_media_mvp.md)
- [群组与频道 MVP](docs/34_groups_channels_mvp.md)
- [更新日志](CHANGELOG.md)
- [安全策略](SECURITY.md)

## 本地测试

```bash
# Smoke test（基础启动验证）
bash scripts/smoke.sh

# 全流程自动化测试（构建、打包、安装模拟）
bash scripts/full_check.sh
```

所有测试使用 `tmp/` 沙箱，不访问真实 Telegram 网络。

## 数据目录安全

`data/` 目录包含敏感数据，**不得提交到代码仓库**：

- `atria.db` — SQLite 数据库
- `secret.key` — 加密密钥（丢失后无法恢复 Session）
- `sessions/` — 加密的 MTProto Session 文件
- `logs/` — 应用日志

项目已包含 `.gitignore` 自动排除这些文件。

## 许可证

Apache-2.0 — 详见 [LICENSE](LICENSE)。

## 免责声明

Atria is not affiliated with Telegram. Atria is designed for legitimate session management only. Users are responsible for complying with Telegram's Terms of Service and applicable laws.
