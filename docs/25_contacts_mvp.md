# 联系人 MVP

## 功能范围

- 联系人页面 `/app/#/contacts`
- 展示当前 Telegram 账号的联系人列表
- 联系人字段：display_name、username、phone（脱敏）、avatar_initial、peer_ref、has_dialog
- 本地搜索 display_name / username / phone
- 点击联系人跳转聊天页

## 明确不支持

- 添加/删除/修改联系人
- 批量导入
- 群成员采集
- 真实头像下载
- Telegram 全局用户名搜索
- 在线状态
- 联系人分组/标签

## 架构

### 后端

```
ClientAdapter.GetContacts(ctx, req) (ContactsResult, error)
    ↓
ChatService.GetContacts(ctx, accountID, forceRefresh) (*ContactsResult, error)
    ↓
handleAPIContacts(c *gin.Context)  →  GET /api/contacts
```

### 中立 DTO

```go
type Contact struct {
    PeerRef     string   // u_<id>
    PeerType    PeerType // user
    DisplayName string
    Username    string
    Phone       string  // 脱敏后
    AvatarText  string  // 首字符
    AccessHash  int64   // 不返回前端
    PeerID      int64   // 不返回前端
}
```

### gotd 实现

使用 `tg.ContactsGetContacts(ctx, 0)` 获取联系人列表。

- hash=0 始终返回完整列表
- 过滤 deleted 和 bot 用户
- `mapContacts` 映射为中立 Contact DTO
- `maskPhone` 脱敏手机号（保留前3后2）

### 联系人与会话关联

`GetContacts` 交叉查询 `chat_peer_cache` 表，判断联系人是否已有会话（`has_dialog`）。

点击联系人时：
- 有会话 → 直接跳转 `/chats/{peer_ref}`
- 无会话 → 也跳转 `/chats/{peer_ref}`（chat 模块会自动处理）

### 手机号脱敏

```
13800138000 → 138******00
```

保留前 3 位和后 2 位，中间用 `*` 替代。不足 6 位的手机号原样返回。

## API

### GET /api/contacts

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `force_refresh` | bool | false | 跳过缓存直接调 Telegram |

响应：

```json
{
  "ok": true,
  "contacts": [
    {
      "peer_ref": "u_12345",
      "display_name": "Alice",
      "username": "alice",
      "phone": "138******00",
      "avatar_initial": "A",
      "has_dialog": true
    }
  ],
  "source": "telegram",
  "stale": false
}
```

错误码复用聊天模块的错误码体系。

## 前端

### 路由

`/app/#/contacts` → `ContactsView.vue`

### 搜索

本地过滤，匹配 display_name / username / phone（不区分大小写）。

### 状态

| 状态 | 条件 | 行为 |
|------|------|------|
| 无账号 | `!currentAccountId` | 显示"请先接入"提示 |
| 加载中 | `isLoading` | 显示 skeleton |
| 错误 | `error` | 显示 ErrorBanner |
| 空列表 | `contacts.length === 0` | 显示空状态 |
| 搜索无结果 | `filteredContacts.length === 0` | 显示搜索空状态 |
| 正常 | 有数据 | 显示联系人列表 |

## 安全

- access_hash 不返回前端（`json:"-"`）
- phone 字段已脱敏
- 不记录完整手机号
- API 复用现有 auth + CSRF 保护

## 缓存策略

联系人数据缓存在 `chat_peer_cache` 表中（`peer_type=user`），复用现有的 peer cache 机制。

1. 首次调用 `GET /api/contacts` → 从 Telegram 获取 → 写入 `chat_peer_cache`
2. 后续调用（`force_refresh=false`）→ 从 `chat_peer_cache` 读取（`source=cache, stale=true`）
3. `force_refresh=true` → 跳过缓存，重新从 Telegram 获取并更新缓存
4. Telegram 不可达时自动回退到缓存

联系人写入 `chat_peer_cache` 后，chat 模块的 `GetMessages` 可以直接通过 `peer_ref` 查找 `access_hash`，无需依赖已有 dialog。

## i18n

联系人页全部文案走 i18n（contacts.title, contacts.count, contacts.search, contacts.noAccount, contacts.empty, contacts.noResults, contacts.hasDialog 等），10 种语言支持。

## 限制

- 不支持分页（Telegram contacts API 一次性返回全部）
- 联系人数量多时前端搜索为纯客户端过滤
