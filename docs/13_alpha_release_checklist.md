# Atria v0.1.0-alpha 发布前最终检查报告

**检查时间：** 2026-06-04

## 1. 版本定位

**版本：** v0.1.0-alpha

**定位：** 首个 alpha 版本，功能已具备 alpha 可试用条件，但仍有以下限制：
- 真实 Telegram 登录需要用户在真实环境中手动验证
- 真实 GitHub Release 自更新需要 tag 和 release 产物
- Docker 镜像未实现
- 群组/频道同步暂缓

**不得宣传为：**
- 稳定版
- 生产可用版

## 2. 自动化验证结果

| 检查项 | 结果 |
|--------|------|
| gofmt -l . | ✅ 无输出 |
| go mod tidy | ✅ 成功 |
| go test ./... -count=1 | ✅ 全部通过 |
| go test ./internal/updater/... | ✅ 39 个测试通过 |
| go test ./internal/update/... | ✅ 9 个测试通过 |
| go test ./internal/server/... | ✅ 46 个测试通过（1 个 Windows 跳过） |
| go build ./... | ✅ 成功 |
| go run ./cmd/atria version | ✅ 输出 0.1.0-dev |
| bash scripts/smoke.sh | ✅ 通过 |
| bash scripts/release.sh | ✅ 5 平台构建成功 |
| SHA256SUMS 自动生成 | ✅ 校验和已生成 |
| bash scripts/full_check.sh | ✅ 8 通过，1 跳过 |
| bash -n scripts/*.sh | ✅ 语法正确 |
| go run ./cmd/atria serve | ✅ 正常启动 |

## 3. Release 产物检查

| 产物 | 状态 |
|------|------|
| atria_linux_amd64.tar.gz | ✅ 存在 |
| atria_linux_arm64.tar.gz | ✅ 存在 |
| atria_windows_amd64.tar.gz | ✅ 存在 |
| atria_windows_arm64.tar.gz | ✅ 存在 |
| atria_darwin_amd64.tar.gz | ✅ 存在 |
| atria_darwin_arm64.tar.gz | ✅ 存在 |
| checksums.txt | ✅ 存在，校验通过 |
| README.md | ✅ 包含在每个包中 |
| LICENSE | ✅ 包含在每个包中 |
| 二进制文件 | ✅ 包含在每个包中 |
| data/ | ✅ 不包含 |
| tmp/ | ✅ 不包含 |
| secret.key | ✅ 不包含 |
| sessions/ | ✅ 不包含 |
| logs/ | ✅ 不包含 |
| *.db | ✅ 不包含 |

## 4. 敏感信息扫描结果

**扫描命令：** `grep -RIn --exclude-dir=.git ... -E "api_hash|password|session|secret.key|..." .`

**扫描结果：**
- ✅ 未发现真实 api_hash
- ✅ 未发现真实手机号
- ✅ 未发现真实验证码/2FA 密码
- ✅ 未发现真实 Session 内容
- ✅ 未发现真实 secret.key 内容
- ℹ️ 测试文件中的 `abcdef0123456789abcdef0123456789` 是假数据，仅用于测试
- ℹ️ 文档中对敏感术语的引用是正常说明

## 5. Git 工作区状态分类

**应进入仓库的文件：**
- README.md（已修改）
- CLAUDE.md（新增）
- LICENSE（新增）
- CHANGELOG.md（新增）
- SECURITY.md（新增）
- CONTRIBUTING.md（新增）
- .gitignore（新增）
- go.mod / go.sum（新增）
- cmd/（新增）
- internal/（新增）
- web/（新增）
- docs/（新增）
- scripts/（新增）
- .github/（新增）

**不应进入仓库且已被 .gitignore 排除：**
- data/（运行时数据）
- tmp/（测试沙箱）
- secret.key（加密密钥）
- *.db / *.sqlite（数据库）
- sessions/（Session 文件）
- logs/（日志）
- atria / atria.exe（构建产物）

**敏感文件状态：** 未发现敏感文件出现在 git status 中。

## 6. 真实 Telegram 登录验证状态

**状态：未执行，需人工验证**

原因：真实 Telegram 登录需要：
- 有效的 Telegram API 凭据
- 真实手机号
- 用户手动输入验证码
- 可能需要 2FA 密码

**验证步骤：** 参见 docs/09_manual_mtproto_login_test.md

## 7. 真实 GitHub Release 上传状态

**状态：未执行，需 tag 和仓库权限**

原因：GitHub Release 上传需要：
- 创建并推送 tag（如 v0.1.0-alpha）
- GitHub Actions 自动触发 Release workflow
- 需要仓库写入权限

**操作步骤：**
1. `git tag v0.1.0-alpha`
2. `git push origin v0.1.0-alpha`
3. 在 GitHub Actions 查看 CI/Release

## 8. Web 自更新真实 Release 验证状态

**状态：未执行，需真实 Release assets**

原因：Web 自更新需要：
- GitHub 上存在 Release
- Release 包含对应平台的产物
- checksums.txt 存在

**验证步骤：** 在真实 Release 发布后，通过 Web 设置页检查更新

## 9. 已知限制

| 限制 | 说明 |
|------|------|
| Docker 镜像 | 未实现，Docker 用户需自行构建或等待后续版本 |
| 群组/频道同步 | 暂缓，留到 Phase 5 |
| Windows 自更新 | 运行中 exe 可能无法直接覆盖 |
| macOS 自更新 | 类似 Windows，可能需要手动重启 |
| 真实 Telegram 登录 | 需要用户在真实环境中手动验证 |

## 10. 发布前人工步骤

1. **人工检查 git diff**：确认所有变更符合预期
2. **人工 commit**：
   ```
   git add .
   git commit -m "feat: prepare Atria v0.1.0-alpha release"
   ```
3. **创建 tag**：
   ```
   git tag v0.1.0-alpha
   ```
4. **推送 tag**：
   ```
   git push origin main
   git push origin v0.1.0-alpha
   ```
5. **等待 GitHub Actions**：查看 CI 和 Release workflow 执行
6. **验证 Release 产物**：下载并验证 checksum
7. **手动 Telegram 登录验证**：在真实环境执行 docs/09 的验证清单

## 11. 不得提交的内容

- `data/` — 运行时数据
- `tmp/` — 测试沙箱
- `secret.key` — 加密密钥
- Session 文件 — 加密的 MTProto Session
- SQLite 数据库 — `*.db`, `*.sqlite`, `*.sqlite3`
- 日志 — `*.log`, `logs/`

## 12. 建议 Commit Message

```
feat: prepare Atria v0.1.0-alpha release

- 管理员初始化与登录（bcrypt 密码哈希）
- API 凭据管理（AES-256-GCM 加密存储）
- MTProto 登录闭环（auth.sendCode / auth.signIn / auth.checkPassword）
- 账号资料同步与 Session 状态检测
- 远端 Logout 与本地 Session 删除
- GitHub Actions CI / Release 多平台构建
- Linux 一键安装脚本（install.sh / uninstall.sh）
- Web 自更新（检查/下载/DryRun/应用）
- 现代化 Web 界面（浅色/深色/跟随系统主题）
- 完整审计日志与敏感信息脱敏
- 全流程自动化测试（full_check.sh）

v0.1.0-alpha: 首个 alpha 版本，真实 Telegram 登录需人工验证。
```
