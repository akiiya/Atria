# Atria 贡献指南

感谢您对 Atria 项目的关注！

## 开发前准备

1. 阅读 [CLAUDE.md](CLAUDE.md) 了解项目约束
2. 阅读 `docs/` 目录下的所有文档
3. 确认当前开发阶段（查看 [docs/05_development_plan.md](docs/05_development_plan.md)）

## 开发环境

- Go 1.22+
- 无需 Node.js、数据库或其他外部依赖
- SQLite 默认，零配置即可启动

```bash
# 构建
go build -o atria ./cmd/atria

# 运行
./atria serve

# 测试
go test ./...
```

## 代码规范

- 使用 `gofmt` 格式化代码
- 使用 `go vet` 检查代码
- 文档和注释中文优先
- 关键技术术语可保留英文（MTProto、Session、API 等）

## 提交前检查

提交 PR 前，请确保：

1. `gofmt -l .` 无输出
2. `go test ./... -count=1` 全部通过
3. `bash scripts/smoke.sh` 通过
4. `bash scripts/full_check.sh` 通过（或仅有合理跳过项）

## 禁止事项

以下内容**不得**提交：

- `data/` 目录（数据库、密钥、Session、日志）
- `secret.key`
- 真实 API Hash
- 真实手机号
- 真实验证码或 2FA 密码
- `tmp/` 目录（测试沙箱）

以下功能**不得**实现：

- 批量消息或垃圾信息
- 批量邀请或加入群组
- 绕过平台限制
- 账号交易或接码平台
- 任何形式的自动化骚扰
- 默认管理员密码
- 重前端框架（React、Vue、Svelte 等）

## PR 要求

- 说明修改内容
- 说明测试结果
- 如果修改了文档行为，同步更新文档
- 不得引入当前阶段之外的功能

## 安全问题

发现安全漏洞请参阅 [SECURITY.md](SECURITY.md)。

## 许可证

贡献的代码将按照 [Apache-2.0](LICENSE) 许可证发布。
