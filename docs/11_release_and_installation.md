# Atria — 发布与安装指南

## 1. 发布目标

Atria 采用 GitHub Release 发布，目标版本 `v0.1.0-alpha`。

**定位：**
- 功能已具备 alpha 可试用条件
- 自动化测试和 smoke test 通过
- 真实 Telegram 登录链路需要用户在真实环境中手动验证
- 不得宣传为稳定版或生产可用版

## 2. 支持平台

| 平台 | 架构 | 文件名 |
|------|------|--------|
| Linux | amd64 | atria_linux_amd64.tar.gz |
| Linux | arm64 | atria_linux_arm64.tar.gz |
| Windows | amd64 | atria_windows_amd64.zip |
| Windows | arm64 | atria_windows_arm64.zip |
| macOS | amd64 | atria_darwin_amd64.tar.gz |
| macOS | arm64 | atria_darwin_arm64.tar.gz |

## 3. Release 产物

每个压缩包包含：
- `atria` 或 `atria.exe` — 单二进制可执行文件
- `README.md` — 项目说明
- `LICENSE` — Apache-2.0 许可证

Release 附加文件：
- `checksums.txt` — SHA-256 校验和
- `install.sh` — Linux 一键安装脚本
- `uninstall.sh` — Linux 卸载脚本

## 4. Linux 一键安装

```bash
# 从 GitHub Release 安装最新版
curl -fsSL https://raw.githubusercontent.com/<owner>/atria/main/scripts/install.sh | bash
```

安装后访问：http://127.0.0.1:8080

**Try-Run 模式（不写入系统路径）：**

```bash
# 下载脚本后本地执行
ATRIA_INSTALL_DRY_RUN=1 bash install.sh
```

## 5. systemd 服务

install.sh 会自动创建 systemd 服务：

- 服务名：atria
- 运行用户：atria（专用系统用户）
- 数据目录：/var/lib/atria
- 日志目录：/var/log/atria
- 监听地址：127.0.0.1:8080

管理命令：
```bash
systemctl status atria
systemctl restart atria
systemctl stop atria
journalctl -u atria -f
```

## 6. 手动安装

```bash
# 下载对应平台的压缩包
tar -xzf atria_linux_amd64.tar.gz

# 运行
./atria serve
```

## 7. Windows 运行

1. 下载 `atria_windows_amd64.zip`
2. 解压
3. 运行 `atria.exe serve`
4. 访问 http://127.0.0.1:8080

## 8. macOS 运行

```bash
# 下载对应架构的压缩包
tar -xzf atria_darwin_arm64.tar.gz

# 运行
./atria serve
```

## 9. 卸载

```bash
# 使用卸载脚本
bash uninstall.sh

# Try-Run 模式
ATRIA_UNINSTALL_DRY_RUN=1 bash uninstall.sh
```

卸载脚本会：
- 停止并禁用 systemd 服务
- 删除二进制文件
- **不删除数据目录**（保留用户数据）

如需彻底删除：
```bash
rm -rf /var/lib/atria
rm -rf /var/log/atria
```

## 10. 数据目录

```
/var/lib/atria/
├── atria.db          # SQLite 数据库
├── secret.key        # 加密密钥
├── sessions/         # 加密的 Session 文件
└── tmp/              # 临时文件
```

**重要：** 请务必备份 `secret.key`。丢失密钥将导致加密的 Session 数据无法恢复。

## 11. 升级

1. 停止服务：`systemctl stop atria`
2. 备份旧二进制：`cp /usr/local/bin/atria /usr/local/bin/atria.bak`
3. 下载新版本并替换
4. 启动服务：`systemctl start atria`

install.sh 会自动备份旧二进制。

## 12. 反向代理

如需公网访问，建议使用反向代理：

**Nginx 示例：**
```nginx
server {
    listen 443 ssl;
    server_name atria.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 13. 真实 Telegram 登录验证

Atria 的 MTProto 登录功能需要用户在真实环境中手动验证：

1. 从 https://my.telegram.org 获取 API 凭据
2. 在 Atria 中创建 API 凭据
3. 使用真实手机号登录
4. 输入 Telegram 发送的验证码
5. 如启用两步验证，输入 2FA 密码

详见 [docs/09_manual_mtproto_login_test.md](09_manual_mtproto_login_test.md)。

## 14. 全流程自动化测试

```bash
bash scripts/full_check.sh
```

测试内容：
- gofmt 检查
- go mod tidy 检查
- go test 运行
- go build 构建
- smoke test
- 多平台构建
- 产物完整性检查（release.sh 自动执行）
- checksum 校验
- install.sh Try-Run（仅 Linux）
- uninstall.sh Try-Run

所有测试使用 `tmp/` 沙箱，不访问真实 Telegram 网络。

### 产物完整性检查

`scripts/release.sh` 构建完成后自动生成 SHA256SUMS，包含 5 个平台产物的校验和。

检查内容：
- 5 个平台产物存在（linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64）
- SHA256SUMS 校验通过
- 每个包包含 atria、README.md、LICENSE
- 每个包不包含 data/、tmp/、secret.key、sessions/、日志

## 15. CI 覆盖

GitHub Actions CI 在 Ubuntu 上执行：
- gofmt 校验
- go mod tidy 校验
- go test 运行
- go build 构建
- 脚本语法检查（bash -n）
- smoke test
- 多平台构建
- 产物完整性检查
- install.sh Try-Run

## 16. GitHub Release

Release 由 tag push 触发（例如 `v0.1.0-alpha`）。

Release 产物：
- 6 个平台压缩包
- checksums.txt
- install.sh
- uninstall.sh

创建 Release 需要 GitHub token 和仓库写入权限。

## 17. Web 自更新

Atria 支持通过 Web 界面检查和应用更新。

### 更新流程
1. 检查 GitHub Release
2. 选择匹配当前平台的产物
3. 下载并校验 checksum
4. 备份当前二进制
5. 替换为新版本
6. 提示重启服务

### 与 install.sh 的关系
- Web 自更新依赖 GitHub Release 产物
- install.sh 也从 GitHub Release 下载
- 两者可以配合使用

### Docker 环境
- Docker 容器内不支持自更新
- 请使用新镜像重建容器：
  ```bash
  docker pull akiiya/atria:latest
  docker-compose down
  docker-compose up -d
  ```

### 权限不足时
- 如果没有写入权限，提示使用 install.sh 或手动替换
- Linux systemd 环境可能需要 sudo

### v0.1.0-alpha 限制
- 自更新功能标记为 alpha
- 真实 GitHub Release 自更新需要 tag 和 release 产物
- Windows 运行中 exe 替换可能受限

### full_check 覆盖
- full_check.sh 包含自更新 Try-Run
- 使用 tmp/ 沙箱，不访问真实 GitHub
- 详见 scripts/full_check.sh

## 18. 非目标

Atria **不支持**：
- 批量登录或批量 Logout
- 批量消息或垃圾信息
- 批量邀请成员或加入群组
- 绕过平台限制
- 账号交易或账号市场
- 手机号采集或接码平台
- 任何形式的自动化骚扰
