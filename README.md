# Atria

> A lightweight self-hosted MTProto session manager built with Go.
>
> 使用 Go 构建的轻量级自托管 MTProto Session 管理器。

**Atria** 是一个轻量、自托管的 MTProto 多账号 Session 管理面板。它通过安全的内嵌 Web 界面，管理多个 Telegram 兼容的 MTProto 用户 Session、API 凭据配置、账号元信息和审计日志。

> **当前版本：v0.1.0-alpha** — 首个 alpha 版本，真实 Telegram 登录需要用户在真实环境中手动验证。

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
- **审计日志** — 追踪所有管理操作的详细审计记录
- **Web 自更新** — 设置页可检查更新、下载、Dry-Run 验证、应用更新（alpha 能力）
- **现代化 Web 界面** — 全屏管理面板，支持浅色/深色/跟随系统主题
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

- 密码使用 bcrypt 或 argon2id 哈希存储（永不保存明文）
- API Hash 在数据库中加密存储
- Session 文件使用本地密钥加密
- 日志中不出现敏感数据（密码、API Hash、Session 内容、验证码）
- 所有状态变更操作提供 CSRF 保护
- Cookie 安全属性（HttpOnly、Secure、SameSite）

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

## 技术栈

- **Go** + **Gin** Web 框架
- **GORM** ORM，默认 SQLite，可选 PostgreSQL / MySQL / MariaDB
- **Go embed** 实现单二进制 Web 资源嵌入
- **Go html/template** 服务端渲染，无 Node.js 构建链

## 项目结构

```
atria/
├── cmd/atria/              # 应用入口
├── internal/
│   ├── config/             # 配置加载
│   ├── crypto/             # 加密工具
│   ├── database/           # 数据库初始化
│   ├── model/              # GORM 数据模型
│   ├── server/             # HTTP 服务器和路由
│   ├── updater/            # 自更新接口（预留）
│   ├── version/            # 版本信息
│   └── web/                # 嵌入式 Web 资源访问
├── web/
│   ├── templates/          # HTML 模板
│   └── static/             # CSS 和静态文件
└── docs/                   # 项目文档
```

## 文档

- [项目范围](docs/01_project_scope.md)
- [Telegram API 调研](docs/02_telegram_api_research.md)
- [架构设计](docs/03_architecture.md)
- [数据模型](docs/04_data_model.md)
- [开发计划](docs/05_development_plan.md)
- [安全策略](docs/06_security_policy.md)
- [UI 设计指南](docs/07_ui_design_guidelines.md)
- [Release 与自更新设计](docs/08_release_and_self_update.md)
- [手动验证指南](docs/09_manual_mtproto_login_test.md)
- [真实环境验证报告](docs/10_real_world_validation_report.md)
- [发布与安装指南](docs/11_release_and_installation.md)
- [Web 自更新](docs/12_web_self_update.md)
- [更新日志](CHANGELOG.md)
- [安全策略](SECURITY.md)
- [贡献指南](CONTRIBUTING.md)

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
