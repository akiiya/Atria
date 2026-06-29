# Atria — 开发计划

## Phase 0：项目初始化与文档

**目标：** 建立项目骨架、文档和开发基础。

**范围：**
- Go 模块初始化
- 目录结构创建
- 核心文档（README、CLAUDE.md、docs/）
- Apache-2.0 许可证
- 数据模型定义（GORM 结构体）
- 配置结构（支持环境变量覆盖）
- 密钥管理骨架
- 数据库初始化骨架（SQLite 已接通，PostgreSQL/MySQL 预留）
- 嵌入式 Web 资源机制（go:embed）
- 最小 Gin 服务器（健康检查和占位页面）
- CLI 骨架（serve、version、help）

**验收标准：**
- `gofmt` 通过
- `go mod tidy` 通过
- `go test ./...` 可执行
- `go run ./cmd/atria serve` 可启动
- `GET /healthz` 返回 JSON 状态
- `GET /` 渲染嵌入式首页
- `GET /init` 渲染嵌入式初始化占位页
- 构建后不依赖外部 web/templates/static 目录
- 6 份文档完整
- LICENSE 存在且为 Apache-2.0
- 无高风险功能
- 无默认密码
- 敏感信息不写入日志

---

## Phase 0.5：产品基线修正、UI 基线、中文化、自更新设计

**目标：** 建立现代化 UI 基线，中文化文档和界面，预留自更新设计。

**范围：**
- 重构 Web 模板为全屏应用布局（侧边栏 + 顶部栏 + 卡片内容区）
- 实现 light / dark / system 主题切换（CSS 变量 + localStorage）
- 新增系统设置页面
- 新增错误页面（403、404、500）
- 文档和界面中文化
- 版本信息包（internal/version）
- 自更新接口预留（internal/updater）
- UI 设计指南文档
- Release 与自更新设计文档

**验收标准：**
- `gofmt` 通过
- `go mod tidy` 通过
- `go test ./...` 可执行
- `go run ./cmd/atria serve` 可启动
- `GET /` 显示全屏应用布局（侧边栏 + 顶部栏 + 卡片）
- `GET /init` 显示全屏初始化页
- `GET /settings` 显示系统设置页
- 页面支持 light / dark / system 主题切换
- 主题偏好保存到 localStorage
- README.md 中文优先
- CLAUDE.md 中文优先
- docs 目录文档中文优先
- 新增 docs/07_ui_design_guidelines.md
- 新增 docs/08_release_and_self_update.md
- 新增 internal/updater/ 接口预留
- 新增 internal/version/ 版本信息
- 无真实高风险功能
- 无真实自更新替换逻辑
- 无重前端框架或 Node.js 构建链

---

## Phase 1：基础框架

**目标：** 完成配置、数据库迁移、加密工具、CSRF 和认证中间件的基础框架。

**范围：**
- 完整配置加载（环境变量、默认值、验证）
- 数据库自动迁移（所有模型）
- 密钥加载和加密工具（AES-256-GCM）
- Session 加密/解密函数
- API Hash 加密/解密函数
- CSRF 中间件
- 基于 Session 的认证中间件（Cookie 管理）
- 完善的错误处理和日志
- 结构化审计日志辅助函数

**验收标准：**
- 启动时自动创建数据库表
- 首次运行时自动生成密钥
- 加密/解密函数正常工作
- CSRF Token 生成和验证
- 所有中间件应用到相应路由

---

## Phase 2：管理员初始化与登录

**目标：** 实现首次管理员设置和登录/登出流程。

**范围：**
- `/init` 页面：管理员用户名 + 密码表单
- 密码哈希（bcrypt 或 argon2id）
- 一次性初始化强制执行
- `/login` 页面：用户名 + 密码表单
- Session Cookie 管理（HttpOnly、SameSite）
- 登出和 Session 失效
- 密码修改功能
- 所有认证事件的审计日志

**验收标准：**
- 首次访问重定向到 `/init`
- 管理员可设置用户名和密码
- 初始化不可重复执行
- 正确凭据可登录
- 错误凭据登录失败（无信息泄露）
- 登出清除 Session
- 密码修改正常
- 所有事件记录到 audit_logs

---

## Phase 3：API 凭据管理

**目标：** 完整的 MTProto API 凭据 CRUD 和风险策略配置。

**范围：**
- API 凭据列表页（脱敏显示哈希）
- 新增凭据表单（display_name、api_id、api_hash）
- 编辑凭据（display_name、status、risk_policy；api_hash 仅允许重填）
- 删除凭据（需确认）
- 启用/禁用切换
- API Hash 加密存储
- 指纹生成用于显示
- 顶部栏快速切换下拉框（持久化到 Session）
- 所有凭据操作的审计日志

**验收标准：**
- 列表显示 display_name、api_id、api_hash 指纹、status、risk_policy
- 新增时加密 api_hash
- 编辑时不暴露完整 api_hash
- 删除需确认
- 快速切换下拉框跨页面工作
- 所有操作记录到 audit_logs

---

## Phase 4：MTProto 登录与 Session 管理

**目标：** 通过 MTProto 实现 Telegram 用户账号登录，加密存储 Session。

### Phase 4.0：接入地基与框架（已完成）

**范围：**
- gotd/td 依赖引入
- MTProto Client interface 定义
- LoginFlow 状态机
- MemoryFlowStore
- FileSessionStore
- FLOOD_WAIT 错误分类预留
- AccountService 骨架
- 账号页面骨架

### Phase 4.1：真实登录闭环（已完成）

**范围：**
- GotdClient.StartLogin 真实调用 auth.sendCode
- GotdClient.SubmitCode 真实调用 auth.signIn
- GotdClient.SubmitPassword 真实调用 auth.checkPassword（SRP）
- 2FA SESSION_PASSWORD_NEEDED 处理
- Per-flow 临时 Session 隔离
- 登录成功后保存正式加密 Session
- 创建或更新 telegram_accounts
- 创建或更新 account_sessions
- 清理临时 Flow 和临时 Session
- 完整审计日志

### Phase 4.2：账号资料同步与 Session 状态检测（已完成）

**范围：**
- GotdClient.SyncProfile 真实调用
- GotdClient.CheckSession 真实调用
- AccountService.SyncProfile
- AccountService.CheckSession
- POST /accounts/:id/sync
- POST /accounts/:id/check-session
- 账号列表和详情页显示 Session 状态、同步时间、检测时间

### Phase 4.3：真实 Logout 与 Session 失效处理（已完成）

**范围：**
- GotdClient.Logout 真实调用 auth.logOut
- AccountService.RemoteLogout（远端注销 + 删除本地文件）
- AccountService.DeleteLocalSession（仅删除本地文件）
- AccountService.HandleSessionInvalid
- AccountService.CleanupExpiredLoginFlows
- POST /accounts/:id/logout（远端 Logout，需确认字段）
- POST /accounts/:id/delete-session（本地删除，需确认字段）
- Session invalid / deleted / error 状态展示
- 重新登录引导
- 完整审计日志

### Phase 4.4：真实环境手动验证与 Bugfix 收口（已完成）

**范围：**
- 补齐 .gitignore
- 新增 scripts/smoke.sh
- 新增 docs/10_real_world_validation_report.md
- 修复 SecretKeyFile 路径问题
- 自动化验证全部通过
- 真实 Telegram 登录验证待人工执行

---

## Phase 5：只读群组/频道信息同步（暂缓）

**目标：** 同步和显示 Telegram 群组/频道元数据。

**状态：** 暂缓，优先进入 Phase 7 发布工程。

**范围：**
- 群组/频道只读同步
- 同步快照存储模型

---

## Phase 6：审计、安全与打包部署

**目标：** 加固安全，完善审计日志，准备部署。

**范围：**
- 完整的审计日志覆盖
- 审计日志查看器页面
- 系统设置页面完整实现
- 安全说明页面
- 速率限制中间件
- 输入验证和清理
- 错误页面模板完善
- 密钥备份文档
- Dockerfile（可选）
- 二进制构建脚本
- 备份/恢复文档

**验收标准：**
- 所有状态变更操作有审计条目
- 审计日志页面可查看和筛选
- 系统设置页面可用
- 速率限制防止滥用
- 错误页面用户友好
- 构建产出单二进制

---

## Phase 7：GitHub Actions、多平台构建、Release、一键安装（已完成）

**目标：** 自动化构建，准备公开发布 v0.1.0-alpha。

**范围：**
- GitHub Actions CI 工作流（ci.yml）
- GitHub Actions Release 工作流（release.yml）
- 多平台构建脚本（scripts/release.sh）
- 版本注入（ldflags）
- checksum 文件生成
- Linux 一键安装脚本（scripts/install.sh）
- Linux 卸载脚本（scripts/uninstall.sh）
- install.sh Try-Run 支持
- 全流程自动化测试（scripts/full_check.sh）
- 发布与安装文档（docs/11_release_and_installation.md）
- README 更新

### Phase 7.1：Release 配置实测与仓库元信息收口（已完成）

**范围：**
- CI 覆盖补强（install.sh Try-Run、产物检查）
- Release workflow 收口（产物上传、Release notes）
- owner/repo 占位符收口
- 仓库元信息文件（CHANGELOG.md、SECURITY.md、CONTRIBUTING.md）
- 产物完整性检查脚本
- 文档更新

---

## Phase 8：Web 自更新能力（已完成）

**目标：** 实现 Web 界面检查更新和应用更新功能。

**范围：**
- GitHub Release API 查询
- 版本比较逻辑
- 系统和 CPU 架构识别
- 对应平台产物下载
- checksum 校验
- 原二进制备份
- 替换当前二进制
- 失败回滚机制
- 更新审计日志
- Web 界面更新状态展示
- Docker 场景检测和提示
- 设置页更新操作（检查、下载、DryRun、应用）

### Phase 8.1：自更新测试与文档补强（已完成）

**范围：**
- updater 单元测试（checksum、asset、github mock、state）
- docs/12_web_self_update.md
- README.md / CLAUDE.md 更新

### Phase 8.2：自更新服务层 / Handler 测试与全流程验证收口（已完成）

**范围：**
- update service 测试（check/download/apply/dry-run/审计）
- update handler / router 测试（登录、CSRF、confirm、Docker）
- Docker unsupported 测试
- full_check.sh 自更新 Try-Run 覆盖
- docs/05、docs/06、docs/11 更新

---

## v0.1.0-alpha 发布前最终检查（待执行）

**目标：** 确认所有功能和测试就绪，准备发布。

**范围：**
- 真实 Telegram 登录验证（需要人工）
- 真实 GitHub Release 验证（需要 tag 和 token）
- 最终文档检查
- 最终安全检查
