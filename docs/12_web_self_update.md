# Atria — Web 自更新

## 1. 目标

Atria 支持通过 Web 界面检查和应用更新，无需用户手动下载和替换二进制文件。

## 2. 支持场景

| 场景 | 支持状态 | 说明 |
|------|----------|------|
| 单二进制部署 | ✅ 支持 | 直接替换二进制并重启 |
| install.sh 安装 | ✅ 支持 | 替换 /usr/local/bin/atria |
| systemd 管理 | ✅ 支持 | 替换后需重启服务 |

## 3. 不支持或限制场景

| 场景 | 状态 | 说明 |
|------|------|------|
| Docker 容器内 | ❌ 禁用 | 请使用新镜像重建容器 |
| Windows 运行中 | ⚠️ 受限 | 运行中的 exe 可能无法直接覆盖 |
| 权限不足 | ⚠️ 受限 | 需要写入权限，否则提示使用 install.sh |
| 非标准安装 | ⚠️ 受限 | 可能需要手动更新 |

## 4. 更新流程

```
检查 GitHub Release
    │
    ▼
选择匹配当前平台的产物
    │
    ▼
下载产物到临时目录
    │
    ▼
校验 SHA-256 checksum
    │
    ▼
解压更新包
    │
    ▼
验证新二进制可执行
    │
    ▼
备份当前二进制
    │
    ▼
替换当前二进制
    │
    ▼
提示重启服务
```

## 5. 回滚策略

- 更新前备份当前二进制到 `data/updates/backups/`
- 替换失败时自动恢复备份
- 用户可手动从备份目录恢复
- 备份文件命名：`atria.bak.{timestamp}`

## 6. 审计日志

| 事件 | action | 风险等级 |
|------|--------|----------|
| 检查更新 | system.update_checked | low |
| 发现新版本 | system.update_available | low |
| 下载完成 | system.update_downloaded | low |
| 应用更新成功 | system.update_applied | high |
| 应用更新失败 | system.update_apply_failed | high |
| DryRun 完成 | system.update_dry_run | low |

## 7. 安全边界

- **不执行远程脚本**：只下载二进制文件，不执行任何脚本
- **不允许任意 URL**：只从配置的 GitHub 仓库下载
- **必须 checksum**：默认强制校验 SHA-256
- **不删除 data/**：更新过程不触碰业务数据
- **不覆盖 secret.key**：加密密钥不会被更新覆盖
- **不删除 Session**：Session 文件不会被删除

## 8. Try-Run 测试

```bash
# 全流程自动化测试（包含自更新 Try-Run）
bash scripts/full_check.sh
```

测试使用 `tmp/` 沙箱：
- Mock Release 放在 `tmp/release-mock/`
- 更新状态保存在 `tmp/` 下的临时目录
- 不访问真实 GitHub Release
- 不写入系统路径

## 9. 人工接手场景

以下情况需要人工介入：

1. **真实 GitHub Release 上传**：需要 GitHub token 和仓库权限
2. **Docker 镜像更新**：需要构建和推送新镜像
3. **systemd 服务重启**：可能需要 root 权限
4. **Windows 服务重启**：需要管理员权限

## 10. 当前限制

- v0.1.0-alpha 阶段，自更新功能标记为 alpha
- 真实 GitHub Release 自更新需要 tag 和 release 产物
- Windows 运行中 exe 替换可能失败
- 不支持自动重启服务

## 11. 普通测试说明

- 普通 `go test` 不访问真实 GitHub Release
- 普通 `go test` 不访问真实 Telegram 网络
- 所有测试使用 mock HTTP server 或本地文件
- 自更新 Try-Run 使用 `tmp/` 沙箱

## 12. Docker 环境

Docker 环境下：
- CheckLatest 仍然可用
- DownloadUpdate 仍然可用
- ApplyUpdate 被禁用，提示"请使用新镜像重建容器"

升级方式：
```bash
docker pull akiiya/atria:latest
docker-compose down
docker-compose up -d
```
