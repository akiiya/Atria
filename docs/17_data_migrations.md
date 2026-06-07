# 数据迁移机制

## 概述

Atria 使用版本化数据迁移机制，在程序启动时自动检测并执行必要的数据升级。这确保了用户升级版本后无需手工操作数据库。

## 工作原理

1. **启动时自动执行**：程序启动时，先执行 GORM AutoMigrate（结构迁移），再执行版本化数据迁移。
2. **版本记录**：已执行的迁移记录在 `data_migrations` 表中，包含版本号和名称。
3. **幂等执行**：已执行的迁移不会重复执行。重复启动程序不会重复修改数据。
4. **安全失败**：迁移失败时程序不会继续启动，避免带着半坏数据运行。
5. **Go 函数编写**：所有迁移使用 Go 函数编写，不使用 SQL 文件。

## 启动流程

```
加载配置 → 初始化日志 → 校验配置 → 创建数据目录
→ 加载加密密钥 → 初始化数据库连接
→ GORM AutoMigrate（结构迁移）
→ 版本化数据迁移（migration.Run）
→ 创建并启动服务器
```

如果任何一步失败，程序立即退出。

## 迁移列表

### 迁移 1：normalize_api_credential_defaults

归一化 API Key 数据，修复 `enabled` / `is_default` 不一致状态。

规则：
- 如果没有任何记录，不创建假 API Key
- 如果存在 `enabled=true` 且 `is_default=true` 的记录，选它作系统 API Key
- 如果不存在默认但有 `enabled` 记录，选第一条设为默认
- 如果全部 `disabled`，不强行启用
- 如果多条 `is_default=true`，只保留一条（ID 最小）
- 不删除任何旧记录

### 迁移 2：init_system_setting_defaults

初始化缺失的系统设置默认值。

初始化项：
- `proxy_enabled` → `false`
- `proxy_type` → `none`
- `proxy_host` → ``
- `proxy_port` → ``
- `proxy_username` → ``
- `proxy_timeout` → `30`
- `proxy_remark` → ``

注意：
- `proxy_password` 缺失时视为空字符串，不写入数据库
- 已存在的配置不会被覆盖

## 开发规范

### 后续版本必须写迁移

任何涉及以下变更的版本都必须新增迁移：

1. 新增数据库字段
2. 修改字段语义
3. 修复历史数据状态
4. 初始化新的系统设置
5. 数据归一化或清理

### 不允许的做法

- 只依赖 GORM AutoMigrate 处理数据语义变化
- 要求用户手工修改数据库
- 使用 SQL 迁移文件
- 迁移中删除用户数据（除非有充分理由并记录）

### 新增迁移步骤

1. 在 `internal/migration/migrations.go` 中注册新迁移：

```go
Register(Migration{
    Version:     3,
    Name:        "your_migration_name",
   	Description: "迁移描述",
    Run:         migration003YourMigration,
})
```

2. 实现迁移函数：

```go
func migration003YourMigration(db *gorm.DB, key []byte) error {
    // 你的迁移逻辑
    return nil
}
```

3. 版本号必须递增且唯一
4. 迁移必须幂等（可重复执行不报错）
5. 迁移必须有测试

### 测试要求

每个迁移都必须有对应的测试，覆盖：
- 正常执行
- 幂等性（重复执行不报错）
- 边界情况（空表、多条记录等）
- 不覆盖已有数据

## 用户须知

- 程序启动时会自动执行数据迁移，无需手动操作
- 迁移日志会显示当前版本和执行的迁移
- 升级版本前建议备份 `data/` 目录和 `secret.key`
- 迁移失败时程序不会启动，需要检查日志
- 迁移日志不会泄露敏感信息（api_hash、proxy_password、Session 等）
