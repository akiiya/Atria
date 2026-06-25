# 30. Accounts/Sessions MVP Enhancement

## 概述

账号会话模块增强，为账号列表提供运行时状态、Session 状态和操作按钮，使账号和运行时管理更清晰可靠。

增强内容包括：
- 账号列表 API 增加运行时状态字段
- 前端账号页面显示运行时状态、Session 状态、错误信息
- 前端账号页面支持启动/停止 Runtime、切换当前账号
- 完整 i18n 支持（10 种语言）

## API 变更

### GET /api/accounts

返回账号列表，增加运行时状态字段。

响应字段变更：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | 账号 ID |
| `display_name` | string | 显示名 |
| `username` | string | 用户名 |
| `user_id` | int64 | Telegram 用户 ID |
| `status` | string | 账号状态（active/banned/logged_out/restricted） |
| `session_status` | string | Session 状态（active/expired/invalid） |
| `runtime_state` | string | 运行时状态（live/syncing/connecting/degraded/offline/stopped/unknown） |
| `last_error` | string | 最近错误信息（脱敏后，可选） |
| `is_current_account` | bool | 是否为当前选中账号 |
| `last_sync` | string | 最后同步时间（YYYY-MM-DD HH:MM） |
| `updated_at` | string | 更新时间（YYYY-MM-DD HH:MM） |
| `has_api_key` | bool | 是否配置了 API Key |

## 状态字段说明

### 账号状态 (status)

| 值 | 说明 |
|----|------|
| `active` | 正常 |
| `banned` | 已封禁 |
| `logged_out` | 已登出 |
| `restricted` | 受限 |

### Session 状态 (session_status)

| 值 | 说明 |
|----|------|
| `active` | 有效 |
| `expired` | 已过期 |
| `invalid` | 无效 |
| 空 | 无 Session |

### 运行时状态 (runtime_state)

| 值 | 说明 |
|----|------|
| `live` | 在线 |
| `syncing` | 同步中 |
| `connecting` | 连接中 |
| `degraded` | 异常 |
| `offline` | 离线 |
| `stopped` | 已停止 |
| `unknown` | 未知（runtimeManager 未初始化） |

## 前端操作

### 可用操作

| 条件 | 操作 | 说明 |
|------|------|------|
| 非当前账号 且 status=active | 选择 (Select) | 切换当前选中账号（表单 POST /accounts/select） |
| 当前账号 且 runtime_state=stopped/offline | 启动 (Start) | 启动 Runtime（POST /api/chats/runtime/start） |
| 当前账号 且 runtime_state=live/syncing/connecting | 停止 (Stop) | 停止 Runtime（POST /api/chats/runtime/stop） |

### UI 布局

- 顶部：页面标题 + 刷新按钮 + 接入账号按钮
- 无 API Key 时：提示配置 API Key
- 无账号时：提示接入账号
- 有账号时：表格展示所有账号信息
  - 当前账号行高亮
  - 当前账号标记圆点
  - 运行时状态用 badge 颜色区分
  - 错误信息截断显示（hover 显示完整）
  - 操作按钮根据状态动态显示

## 安全边界

- 所有 API 端点需要管理员认证
- 运行时操作端点需要 CSRF 校验
- `last_error` 通过 `security.SanitizeErrorMessage()` 脱敏，移除文件路径、API hash、代理密码、手机号等敏感信息
- 账号选择通过 form POST（非 AJAX），携带 CSRF token
- 运行时启动/停止通过 JSON API，携带 CSRF token

## 审计事件类型

已有审计事件（复用现有）：
- `runtime.start` - 启动运行时
- `runtime.stop` - 停止运行时
- `account.select` - 切换账号

## i18n

账号相关 i18n 键以 `accounts.*` 为前缀，支持全部 10 种语言：
en, zh-CN, zh-TW, ja, ko, de, fr, es, pt-BR, ru

主要键分组：
- `accounts.title` / `accounts.desc` - 页面标题
- `accounts.status.*` - 账号状态标签
- `accounts.session*` - Session 状态标签
- `accounts.runtime.*` - 运行时状态标签
- `accounts.startRuntime` / `accounts.stopRuntime` - 操作按钮
