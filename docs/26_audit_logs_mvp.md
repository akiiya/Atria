# 审计日志 MVP

## 功能范围

- 审计日志页面 `/app/#/audit`
- 按 event_type / account_id / risk_level / 时间范围筛选
- 分页浏览（limit + offset）
- 已接入 27+ 个审计事件

## 已接入的审计事件

| 事件 | 来源 | 风险等级 |
|------|------|---------|
| `admin.login` / `admin.logout` / `admin.init` | admin_handler | medium |
| `api_credential.create` / `.update` / `.delete` | credential_handler | medium |
| `account.login_start` / `.code_sent` / `.code_failed` / `.password_required` / `.password_failed` / `.login_authorized` | account_handler | medium-high |
| `account.select` | settings_handler | low |
| `runtime.start` / `runtime.stop` | api_handler | low |
| `settings.proxy.save` | settings_handler | medium |
| `chat.send_message` | chat_handler | low |

## 数据库迁移

Migration 11: 为 `audit_logs` 表添加 `account_id` 字段（INTEGER DEFAULT 0）。

## Audit Logs API

### GET /api/audit

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 50 | 每页条数，最大 200 |
| `offset` | int | 0 | 偏移量 |
| `action` | string | - | 按事件类型筛选 |
| `account_id` | int | - | 按账号 ID 筛选 |
| `risk_level` | string | - | 按风险等级筛选 |
| `since` | string | - | 起始时间（ISO 格式） |
| `until` | string | - | 结束时间（ISO 格式） |

响应：

```json
{
  "ok": true,
  "logs": [...],
  "total": 150,
  "limit": 50,
  "offset": 0
}
```

## 隐私边界

**不记录的敏感内容：**
- 明文密码、API Hash、Session 路径、token、验证码、2FA 密码
- 消息正文（只记录 text_len）
- access_hash 明文
- proxy_password
- CSRF token

**自动脱敏（17 类敏感 key）：**
- `audit.Log` 的 Metadata 中，包含以下敏感关键词的字段自动替换为 `***REDACTED***`：
  - password, password_hash, api_hash, session, token, code, two_factor
  - secret, secret_key, csrf_token, cookie, authorization
  - access_hash, file_reference, local_path, message_body, search_keyword
- 匹配规则：key 名称包含敏感关键词（大小写不敏感，子串匹配）

## 前端页面

- 筛选：事件类型下拉、风险等级下拉、账号 ID 输入
- 分页：上一页/下一页，显示 offset-range / total
- 表格：时间、操作、资源、账号、等级、IP、说明
- 空状态、加载中、错误状态

## 已知限制

- 不支持导出、报表、实时推送、长期归档
- 不支持全文搜索
- actor 为 admin/system（无多用户体系）
- 事件类型硬编码在前端下拉（无后端枚举 API）
