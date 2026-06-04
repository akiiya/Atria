# Atria 更新日志

本文件记录 Atria 的版本更新历史。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)。

## [v0.1.0-alpha] - 开发中

首个 alpha 版本，包含完整的 MTProto Session 管理基础能力。

### 新增

**管理员系统：**
- 管理员初始化（首次访问时设置用户名和密码）
- 管理员登录/登出
- 密码修改
- bcrypt 安全哈希存储

**API 凭据管理：**
- API 凭据 CRUD（新增、编辑、删除）
- API Hash AES-256-GCM 加密存储
- API Hash 脱敏显示
- 顶部栏快速切换当前凭据
- 风险策略配置（disabled / enabled / confirm）

**MTProto 登录：**
- 手机号验证码登录（auth.sendCode）
- 验证码提交（auth.signIn）
- 两步验证支持（auth.checkPassword + SRP）
- 加密 Session 文件存储
- 登录流程状态机（三阶段）

**账号管理：**
- 账号列表页
- 账号详情页
- 账号资料同步
- Session 状态检测
- 远端 Logout（auth.logOut）
- 本地删除 Session
- Session 生命周期管理

**Web 界面：**
- 现代化全屏管理面板
- 浅色/深色/跟随系统主题
- 中文优先界面
- 错误页面（403、404、500）

**安全：**
- CSRF 保护
- 审计日志（敏感字段自动脱敏）
- Cookie 安全属性
- 配置校验

**基础设施：**
- 单二进制部署（Go embed）
- SQLite 默认数据库
- PostgreSQL / MySQL / MariaDB 预留
- GitHub Actions CI
- GitHub Actions Release（多平台构建）
- Linux 一键安装脚本
- 全流程自动化测试

### 当前限制

- 真实 Telegram 登录需要用户在真实环境中手动验证
- 不支持批量登录
- 不支持消息群发
- 不支持群组/频道同步
- 不支持刷量或绕过平台限制
- Web 自更新尚未实现
