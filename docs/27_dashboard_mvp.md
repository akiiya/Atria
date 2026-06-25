# 27. Dashboard MVP

## 概述

Dashboard MVP 为 Vue SPA 仪表盘提供增强的后端 API 支持，包含运行时状态统计、审计日志查询和事件类型列表。

## API 端点

### GET /api/dashboard/stats

返回仪表盘统计数据，已有的字段保持不变，新增以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `runtime_live` | int | 运行时状态为 live 或 syncing 的账号数 |
| `runtime_offline` | int | 运行时状态为 connecting 或 degraded 的账号数 |
| `runtime_stopped` | int | 运行时状态为 stopped 或 offline 的账号数 |
| `recent_errors` | int64 | 近 24 小时内 risk_level 为 high 或 critical 的审计事件数 |
| `recent_audit` | int64 | 近 24 小时内的审计事件总数 |
| `recent_logs` | array | 最近 5 条审计日志 |

`recent_logs` 数组元素结构：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | 日志 ID |
| `action` | string | 事件类型 |
| `message` | string | 事件描述 |
| `risk_level` | string | 风险等级（low/medium/high/critical） |
| `created_at` | string | 创建时间（格式：2006-01-02 15:04:05） |

### GET /api/audit/event-types

返回已使用的审计事件类型列表。

响应结构：

```json
{
  "ok": true,
  "event_types": [
    {
      "value": "admin.login",
      "label": "管理员登录"
    }
  ]
}
```

### GET /api/audit

已有的审计日志查询 API，新增 `event_type` 查询参数作为 `action` 的别名。

`event_type` 优先级高于 `action`，两者同时存在时使用 `event_type`。

## Runtime 状态分类

Dashboard 统计中的 runtime 状态分类：

| Dashboard 分类 | Runtime 状态 | 说明 |
|---------------|-------------|------|
| `runtime_live` | live, syncing | 正常运行 |
| `runtime_offline` | connecting, degraded | 连接中或降级 |
| `runtime_stopped` | stopped, offline | 已停止或离线 |

## 测试

所有新增功能均有对应的单元测试覆盖：

- `TestDashboardStats_IncludesRuntimeAndAudit`：验证 dashboard stats 包含新增字段
- `TestDashboardStats_RecentLogsSortedDesc`：验证最近日志按 ID 降序排列
- `TestAuditEventTypes_ReturnsDistinctActions`：验证事件类型列表包含中文标签
- `TestAuditAPI_SupportsEventTypeAlias`：验证 event_type 参数作为 action 别名
