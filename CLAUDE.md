# Atria — Claude Code 项目指令

## 项目目标

Atria 是一个轻量、自托管的 MTProto 多账号 Session 管理面板，使用 Go 开发。通过安全的内嵌 Web 界面管理多个 Telegram 兼容的 MTProto 用户 Session、API 凭据配置、账号元信息和审计日志。

## Claude Code 角色

Claude Code 是本项目的实现工程师，必须：

- 遵循 `docs/` 目录中记录的架构和设计决策
- 按照规格实现代码，不得重新设计系统
- 偏离文档规格前必须先询问
- 报告冲突或风险，而非擅自做决定

Claude Code **不得**：

- 重新定义项目范围或产品方向
- 引入当前阶段计划之外的功能
- 更改技术栈（Gin、GORM、SQLite、Go embed）
- 添加高风险或批量操作功能
- 创建默认管理员密码或暴露敏感数据
- 实现外部 REST API Key 或多租户系统
- 将 UI 退化为临时 demo 页面
- 破坏 Go embed 单二进制约束

## 每次开发前必须

1. 阅读 `docs/` 目录下的所有文档，了解当前阶段和需求
2. 检查 `docs/05_development_plan.md` 确认当前阶段范围
3. 实现认证或加密代码前先查看 `docs/06_security_policy.md`
4. 不要假设前一阶段的功能自动延续，除非明确说明

## 文档与注释

- 文档和代码注释以中文优先
- 关键技术术语可保留英文，如 MTProto、Session、API Credential、GitHub Actions、Release、SQLite、GORM、Gin、Go embed
- UI 页面文案中文优先，可保留必要英文术语

## 代码风格

- 使用标准 Go 规范（`gofmt`、`go vet`）
- 遵循仓库中已有的代码模式
- 使用 `log/slog` 进行结构化日志（不使用第三方日志库）
- 包名和文件名描述性命名，避免缩写
- 为导出类型和非显而易见的逻辑添加注释
- 函数保持聚焦，优先使用组合而非大型处理器

## UI 约束

- Web 界面必须保持现代化全屏应用布局
- 必须支持 light / dark / system 三种主题模式
- 不得退化为"页面中间放一个小卡片"的临时 demo 页面
- 不得引入 React / Vue / Svelte / Tailwind / Bootstrap
- 不得引入 Node.js 构建链
- 不得引入外部 CDN、在线字体或在线图标库

## 安全约束

- **密码：** 使用 bcrypt 或 argon2id 哈希。永不存储、记录或显示明文密码。
- **API Hash：** 数据库中加密存储（AES-256-GCM）。永不记录或显示完整哈希。使用指纹用于显示。
- **手机号：** 数据库中加密存储（AES-256-GCM）。永不记录明文手机号。
- **Session：** 加密 Session 文件。数据库中不存储原始 Session 数据。不记录 Session 内容。
- **验证码 / 2FA：** 永不记录这些值。
- **审计日志：** 不在 `metadata_json` 中写入敏感原始数据。敏感字段自动替换为 `***REDACTED***`。
- **CSRF：** 所有状态变更端点必须有 CSRF 保护。
- **Cookie：** 使用 HttpOnly、Secure（启用 TLS 时）和 SameSite 属性。
- **Web Session：** 使用 AES-256-GCM 加密，token 中不包含明文用户信息。
- **日志脱敏：** 敏感字段（password、api_hash、session、token、code、secret 等）不得进入日志、审计 metadata 或模板。
- **测试覆盖：** 所有安全工具函数（加密、解密、指纹、Session、审计过滤）必须有单元测试。

## 单二进制约束

所有 Web 资源（模板、CSS、JS）必须通过 `go embed` 在构建时嵌入。编译后的二进制文件运行时不得依赖任何外部 `web/`、`templates/`、`static/` 或 `assets/` 目录。

## 数据库约束

- 默认数据库为 SQLite（通过 GORM）
- PostgreSQL、MySQL 和 MariaDB 必须通过驱动抽象支持
- 业务代码中不得编写 SQLite 特定逻辑
- 使用环境变量（`ATRIA_DB_DRIVER`、`ATRIA_DB_DSN`）切换数据库

## 数据迁移约束

- 程序启动时自动执行版本化数据迁移（`internal/migration/`）
- 迁移使用 Go 函数编写，不使用 SQL 文件
- 迁移版本记录在 `data_migrations` 表中
- 后续任何版本的数据结构或数据语义变化都必须新增 migration
- 不允许只依赖 GORM AutoMigrate 处理数据语义变化
- 迁移必须幂等（可重复执行不报错）
- 迁移失败时程序不会继续启动
- 迁移必须有测试
- **不支持多个 Atria 进程同时操作同一 data 目录**
- 详见 `docs/17_data_migrations.md`

## 代理集成约束

- API 网络代理配置会用于 Telegram MTProto 登录流程
- GotdClient 通过 `dcs.Plain` resolver 注入自定义拨号函数
- SOCKS5 和 HTTPS CONNECT 代理均支持无认证
- proxy_password 缺失时视为空字符串
- 代理连接失败、认证失败、Telegram 超时返回明确错误
- 自动化测试不得访问真实 Telegram

## Web Embed 约束

- 使用 `embed.FS` 包含 `web/templates/` 和 `web/static/`
- 从嵌入的文件系统解析模板
- 从嵌入的文件系统提供静态文件
- 运行时不得引用外部文件路径

## 自更新约束

- 自更新功能必须经过设计、校验、回滚、审计
- 不得粗暴覆盖二进制文件
- Docker 场景下不建议容器内自更新
- 更新前必须备份原二进制文件
- 必须验证 checksum
- 更新失败必须支持回滚

## 账号生命周期约束

- 必须区分"远端 Logout"和"本地删除 Session"两种操作
- 远端 Logout 必须调用 MTProto auth.logOut，成功后删除本地 Session
- 远端 Logout 失败时不得删除本地 Session（如 FLOOD_WAIT）
- 本地删除 Session 不得调用 Telegram 远端
- 所有状态变更操作必须使用 POST，不得使用 GET
- 所有 POST 必须校验 CSRF
- 所有危险操作必须要求服务端确认字段
- 不得实现批量 Logout
- Session invalid / deleted / error 状态必须有友好提示和重新登录引导
- secret.key 丢失或更换会导致旧 Session 无法解密，这是预期行为

## 发布工程约束

- **Git tag 是唯一版本来源**，不使用 VERSION 文件
- 正式版本号 = tag 去掉 `v` 前缀，通过 ldflags 注入
- 日常/CI 构建版本号由 `git describe` 派生
- 发版动作 = 打新 `v*` tag（命令行或 GitHub 网页），由 Actions 自动完成
- 分支模型：main 受保护（必须 PR、禁止 force push），dev 开发
- 本地构建：`make build` 或 `bash scripts/build.sh`
- 发布脚本不得删除用户数据
- 安装脚本不得生成默认管理员
- Release 包不得包含 data/、tmp/、secret.key、Session、日志
- Docker 中不得包含用户数据
- 全流程测试优先自动化，只有必须人工参与才请求人工接手
- 所有测试临时文件优先放在根目录 tmp/
- install.sh 不得覆盖已有 secret.key
- install.sh Try-Run 不得写入真实系统路径
- Release 产物通过 `scripts/release.sh` 自动检查（SHA256SUMS）
- CI 必须在 Ubuntu 上执行 install.sh Try-Run
- 只有真实 GitHub token / 真实 Telegram 凭据 / root systemd 权限才允许请求人工

## 自更新约束

- 自更新不得执行远程脚本
- 自更新必须校验 checksum
- 自更新不得允许任意 URL
- 自更新不得删除用户数据
- 自更新不得删除 secret.key
- 自更新不得删除 Session
- 自更新测试必须走 tmp/ mock release
- Docker 环境 apply 必须禁用
- Updater 相关代码必须有单元测试

## 禁止事项

- 实现批量消息、邀请或群组操作
- 添加手机号接码平台
- 创建账号交易或市场功能
- 绕过 Telegram 平台限制或速率限制
- 添加默认管理员凭据（admin/admin 等）
- 记录敏感值（密码、API Hash、Session、验证码、2FA）
- 引入 Node.js、React、Vue、Svelte 或任何前端构建链
- 创建外部 REST API Key 或多租户系统
- 为方便而移除或削弱安全控制
- 实现批量 Logout
- 让普通自动化测试访问真实 Telegram 网络
- 绕过 FLOOD_WAIT
- 实现群组/频道同步（Phase 5）

## 开发流程

1. 检查 `docs/05_development_plan.md` 确认当前阶段
2. 仅实现当前阶段要求的内容
3. 报告完成前运行 `gofmt`、`go vet`、`go test ./...`
4. 如果实现改变了文档中记录的行为，更新文档
5. 报告已实现内容、剩余内容和任何风险

## 常用命令

```bash
# 构建
go build -o atria ./cmd/atria

# 运行
./atria serve

# 测试
go test ./...

# 格式化
gofmt -w .

# 检查
go vet ./...
```
