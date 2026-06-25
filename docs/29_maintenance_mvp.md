# 29. Data/Cache Maintenance MVP

## 概述

数据维护 MVP 提供系统数据状态查看和缓存清理能力，包含以下功能：

- 数据库表统计（账号、API 密钥、Peer 缓存、消息缓存、审计日志）
- 媒体缓存统计（记录数、已缓存、失败、总大小）
- 孤立缓存检测与清理（没有对应活跃账号的缓存条目）
- 聊天缓存清理（按账号或 peer 维度清理）
- 媒体缓存清理（支持仅清理失败/过期记录）
- 迁移版本显示
- 最近维护操作记录

## API 端点

### GET /api/maintenance/status

返回系统维护状态，需要认证。

响应字段：
- `account_count` - Telegram 账号总数
- `api_key_count` - API 凭据总数
- `peer_cache_count` - Peer 缓存总数
- `message_cache_count` - 消息缓存总数
- `audit_log_count` - 审计日志总数
- `orphan_peers` - 孤立 Peer 缓存数（没有对应 active 账号）
- `orphan_messages` - 孤立消息缓存数（没有对应 active 账号）
- `migration_version` - 当前数据迁移版本
- `media_record_count` - 媒体缓存记录总数
- `media_cached_count` - 已缓存媒体数
- `media_failed_count` - 失败媒体数
- `media_total_size` - 媒体缓存总大小（bytes）
- `recent_maintenance` - 最近 5 条维护操作审计记录

### POST /api/maintenance/cleanup/chat-cache

清理指定账号或 peer 的聊天缓存，需要认证 + CSRF。

请求体：
```json
{
  "account_id": 1,
  "peer_ref": "u_123456",
  "dry_run": true
}
```

- `account_id`（必填）- 账号 ID
- `peer_ref`（可选）- 指定 peer，为空则清理该账号所有缓存
- `dry_run`（可选）- 默认 true，显式传 false 才执行删除

dry-run 模式下只返回将删除的数量，不实际删除。执行模式下会写入审计日志。

### POST /api/maintenance/cleanup/media-cache

清理媒体缓存文件和元数据，需要认证 + CSRF。

请求体：
```json
{
  "account_id": 1,
  "peer_ref": "u_123456",
  "only_failed": true,
  "dry_run": true
}
```

- `account_id`（可选）- 账号 ID，为空则清理所有
- `peer_ref`（可选）- 指定 peer
- `only_failed`（可选）- 仅清理 failed/downloading 状态的记录
- `dry_run`（可选）- 默认 true，显式传 false 执行删除

dry-run 模式下返回缓存统计。执行模式下会删除文件和数据库记录，并写入审计日志。

### POST /api/maintenance/cleanup/orphans

清理孤立缓存（没有对应活跃账号的缓存条目），需要认证 + CSRF。

请求体：
```json
{
  "dry_run": true
}
```

- `dry_run`（可选）- 默认 true，显式传 false 执行删除

## 安全边界

- 所有维护端点需要管理员认证
- 写操作端点需要 CSRF 校验
- 删除操作默认 dry-run，必须显式传 `dry_run: false` 才执行
- 所有实际删除操作记录审计日志（action: `maintenance.cleanup_chat_cache` / `maintenance.cleanup_orphans`）
- 审计日志中不包含敏感数据

## 审计事件类型

- `maintenance.cleanup_chat_cache` - 清理聊天缓存
- `maintenance.cleanup_orphans` - 清理孤立缓存
- `maintenance.cleanup_media_cache` - 清理媒体缓存

## 前端

- 路由: `/maintenance`
- 侧边栏入口: 导航栏 "数据维护"
- 支持 light / dark / system 主题
- 表统计卡片展示
- 孤立缓存预览/执行
- 聊天缓存按账号/peer 清理
- 最近维护记录表格

## i18n

维护相关 i18n 键以 `maintenance.*` 为前缀，支持全部 10 种语言：
en, zh-CN, zh-TW, ja, ko, de, fr, es, pt-BR, ru

审计事件类型键以 `event.maintenance.*` 为前缀。
