# Atria — Release 与自更新设计

## 1. Release 目标

Atria 托管在 GitHub，通过 GitHub Actions 自动构建和发布。

目标：
- 每次推送 tag（如 `v1.0.0`）自动触发构建
- 生成多平台二进制文件
- 创建 GitHub Release 附带产物
- 提供一键安装脚本
- 支持 Web 界面检查和应用更新

## 2. GitHub Actions 构建策略

构建流程：

```
触发条件：推送 tag (v*)
    │
    ├── 运行测试 (go test ./...)
    │
    ├── 多平台并行构建
    │   ├── linux/amd64
    │   ├── linux/arm64
    │   ├── darwin/amd64
    │   ├── darwin/arm64
    │   └── windows/amd64
    │
    ├── 生成 checksum 文件
    │
    ├── 创建 GitHub Release
    │   ├── 上传所有二进制文件
    │   ├── 上传 checksum 文件
    │   └── 自动生成 changelog
    │
    └── 触发安装脚本更新（可选）
```

构建命令示例：

```bash
CGO_ENABLED=0 go build \
  -trimpath -ldflags "-s -w \
    -X github.com/user/atria/internal/version.Version=${VERSION}" \
  -o atria-${OS}-${ARCH}${EXT} \
  ./cmd/atria
```

## 3. 多平台产物命名规范

命名格式：`atria-{os}-{arch}[.exe]`

| 平台 | 架构 | 文件名 |
|------|------|--------|
| Linux | amd64 | atria-linux-amd64 |
| Linux | arm64 | atria-linux-arm64 |
| macOS | amd64 | atria-darwin-amd64 |
| macOS | arm64 | atria-darwin-arm64 |
| Windows | amd64 | atria-windows-amd64.exe |

## 4. Checksum 文件设计

每个 Release 附带 `checksums.txt` 文件：

```
abc123def456...  atria-linux-amd64
789abc012def...  atria-linux-arm64
456def789abc...  atria-darwin-amd64
012abc345def...  atria-darwin-arm64
def789abc012...  atria-windows-amd64.exe
```

校验算法：SHA-256

## 5. 签名校验预留

预留 GPG 签名支持：

- 构建时可选生成 `.sig` 签名文件
- 更新时可选验证签名
- 当前阶段不强制要求签名

后续可扩展：
- GitHub Actions 使用 secrets 存储 GPG 私钥
- 每个产物附带 `.sig` 文件
- 安装脚本和自更新逻辑验证签名

## 6. 一键安装脚本规划

提供 `install.sh` 脚本（Linux/macOS）：

```bash
#!/bin/bash
# Atria 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/user/atria/main/install.sh | bash

REPO="user/atria"
INSTALL_DIR="/usr/local/bin"

# 检测平台和架构
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# 映射架构名称
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

# 获取最新版本
LATEST=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

# 下载并安装
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST}/atria-${OS}-${ARCH}"
curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/atria"
chmod +x "${INSTALL_DIR}/atria"

echo "Atria ${LATEST} installed to ${INSTALL_DIR}/atria"
```

Windows 用户可使用 PowerShell 脚本或手动下载。

## 7. Web 自更新流程

Web 界面提供更新入口，流程如下：

```
用户点击"检查更新"
    │
    ├── 调用 GitHub API 获取最新 Release
    │
    ├── 比较版本号
    │   ├── 当前版本 >= 最新版本 → 提示"已是最新"
    │   └── 当前版本 < 最新版本 → 显示更新信息
    │
    ├── 显示 Release 说明和下载按钮
    │
    ├── 用户确认后开始更新
    │   ├── 下载对应平台的二进制文件
    │   ├── 验证 checksum
    │   ├── 备份当前二进制到 data/backups/
    │   ├── 替换当前二进制
    │   └── 重启服务（或提示用户手动重启）
    │
    └── 记录更新审计日志
```

## 8. 回滚策略

更新失败时的回滚机制：

1. **备份原二进制** — 更新前将当前二进制复制到 `data/backups/atria-{version}-{timestamp}`
2. **替换失败回滚** — 如果新二进制无法启动，自动恢复备份
3. **手动回滚** — 用户可从 `data/backups/` 目录手动恢复
4. **保留最近 3 个备份** — 自动清理旧备份

## 9. 审计日志要求

所有更新操作必须记录审计日志：

| 事件 | action | 说明 |
|------|--------|------|
| 检查更新 | `system.check_update` | 记录检查结果 |
| 开始更新 | `system.update_start` | 记录目标版本 |
| 更新成功 | `system.update_success` | 记录新旧版本 |
| 更新失败 | `system.update_failed` | 记录错误信息 |
| 回滚操作 | `system.update_rollback` | 记录回滚原因 |

## 10. Linux systemd 场景

当 Atria 作为 systemd 服务运行时：

1. 更新二进制文件
2. 通过 `systemctl restart atria` 重启服务
3. 如果重启失败，自动回滚并重启旧版本

systemd 服务文件示例：

```ini
[Unit]
Description=Atria MTProto Session Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/atria serve
WorkingDirectory=/opt/atria
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## 11. Windows 服务场景

Windows 下的更新策略：

1. 更新二进制文件
2. 如果作为 Windows 服务运行，需要重启服务
3. 如果作为普通进程运行，提示用户手动重启

注意：Windows 下替换正在运行的二进制文件可能失败，需要：
- 先将当前二进制重命名
- 再将新二进制移动到原位置
- 最后重启进程

## 12. macOS 场景

macOS 下的更新与 Linux 类似：

1. 下载 darwin-amd64 或 darwin-arm64 版本
2. 替换二进制文件
3. 重启进程或服务

如果使用 launchd 管理服务，需要通过 `launchctl` 重启。

## 13. Docker 场景限制

**Docker 容器内不建议使用自更新功能。**

原因：
- 容器内的文件修改不会持久化
- 容器重启后会丢失更新
- 违背容器不可变原则

建议做法：
- 使用新镜像重建容器
- 使用 Docker Compose 管理版本
- 在 Web 界面提示"Docker 部署请使用新镜像重建"

Web 界面应检测是否运行在容器内（检查 `/.dockerenv` 或 cgroup），如果是则禁用自更新按钮并显示提示。

## 14. 安全风险说明

自更新功能的安全考虑：

1. **下载安全** — 必须使用 HTTPS 下载
2. **完整性校验** — 必须验证 checksum
3. **签名验证** — 预留 GPG 签名验证
4. **权限问题** — 更新需要写入二进制文件的权限
5. **回滚能力** — 必须保留备份以应对失败
6. **审计追踪** — 所有操作必须记录日志

风险缓解：
- 只从 GitHub 官方 API 获取 Release 信息
- 只从 GitHub 官方 CDN 下载产物
- 校验和验证失败则中止更新
- 更新失败自动回滚

## 15. 本阶段说明

**本阶段（Phase 0.5）仅完成：**
- 设计文档
- 接口定义（`internal/updater/`）
- Web 界面占位

**不实现：**
- 真实的 GitHub API 调用
- 真实的文件下载
- 真实的二进制替换
- 真实的回滚逻辑

**实现计划：**
- Phase 7 — GitHub Actions 构建和 Release 流程
- Phase 8 — Web 自更新能力实现
