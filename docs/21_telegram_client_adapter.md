# Telegram Client Adapter 架构

## 为什么引入 Telegram Client Adapter

Atria 未来 100% 会从 gotd 切换到 TDLib。如果不建立清晰的适配器边界，gotd 类型会渗透到业务层（chat、server、cache、API DTO），未来切换时需要重写大量代码。

## 为什么假设未来会切 TDLib

- gotd/td 是社区维护的纯 Go 实现，长期维护活跃度不确定
- TDLib 是 Telegram 官方推荐的客户端库，功能更完整
- TDLib 对长连接、实时更新的支持更成熟

## 为什么本轮不切 TDLib

- TDLib 需要 CGO 或 pre-built native library，部署复杂度高
- 当前 gotd 实现已能满足基本聊天需求
- 切换 TDLib 需要重新测试所有 Telegram 交互
- 本轮目标是建立边界，不是切换实现

## 目录结构

```
internal/telegramclient/
├── adapter.go          # ClientAdapter 接口定义
├── types.go            # 中立 DTO（Dialog, Message, Media 等）
├── errors.go           # 中立错误码（ErrorCode, Error）
├── runtime.go          # RuntimeManager 接口 + UpdateEvent DTO
├── event_bus.go        # EventBus 内部事件总线
├── boundary_test.go    # 架构边界测试
├── event_bus_test.go   # EventBus 测试
├── gotd/
│   ├── adapter.go      # gotd 实现的 ClientAdapter
│   ├── mapper.go       # gotd 类型到中立 DTO 的映射
│   ├── mapper_test.go  # mapper 单元测试
│   ├── runtime.go      # gotd RuntimeManager 实现
│   ├── runtime_test.go # runtime 测试
│   ├── state_store.go  # updates.StateStorage 实现（SQLite）
│   ├── hash_store.go   # updates.ChannelAccessHasher 实现
│   ├── update_handler.go # telegram.UpdateHandler 实现
│   └── update_mapper_test.go # update mapper 测试
└── tdlib/
    └── README.md       # 未来 TDLib adapter 设计说明
```

## ClientAdapter 接口

```go
type ClientAdapter interface {
    ListDialogs(ctx context.Context, req ListDialogsRequest) (DialogsPage, error)
    GetRecentMessages(ctx context.Context, req GetRecentMessagesRequest) (MessagesPage, error)
    LoadOlderMessages(ctx context.Context, req LoadOlderMessagesRequest) (MessagesPage, error)
    SendText(ctx context.Context, req SendTextRequest) (SendResult, error)
}
```

所有请求和返回类型使用中立 DTO，不包含任何 gotd 类型。

## 中立 DTO

定义在 `types.go`：
- `Dialog` — 会话信息
- `Message` — 消息信息
- `Media` — 媒体信息
- `PeerType` — 会话类型（user/chat/channel）
- `MessageKind` — 消息类型（text/photo/document 等）
- `MessageDirection` — 消息方向（in/out）
- `DataSource` — 数据来源（cache/telegram/mixed）

## 中立错误码

定义在 `errors.go`：
- `ErrorCodeSessionInvalid` — Session 失效
- `ErrorCodePeerInvalid` — 会话无效
- `ErrorCodeAPIKeyInvalid` — API Key 无效
- `ErrorCodeFloodWait` — 请求限流
- `ErrorCodeTelegramTimeout` — 连接超时
- 等等

上层业务只判断 `telegramclient.ErrorCode`，不解析 gotd tgerr。

## RuntimeManager 接口

定义在 `runtime.go`：

```go
type RuntimeManager interface {
    StartAccount(accountID uint) error
    StopAccount(accountID uint) error
    Status(accountID uint) RuntimeStatus
    Subscribe(accountID uint, sink UpdateSink) (Subscription, error)
}
```

gotd 实现在 `gotd/runtime.go`（`RuntimeManagerImpl`）。

每个 active Telegram account 有一个 `AccountRuntime`，持有长-lived 的 `telegram.Client` + `updates.Manager`。详见 `docs/22_account_runtime_updates.md`。

## 内部 EventBus

定义在 `event_bus.go`：

- `Subscribe(accountID, sink)` → 返回 Subscription
- `Publish(accountID, event)` → 非阻塞
- 每个 subscriber 有独立 buffered channel
- 慢 subscriber 不阻塞 runtime
- 下一轮 WebSocket 会接这个 EventBus

## gotd adapter 职责

- 调用 gotd API（MessagesGetDialogs、MessagesGetHistory、MessagesSendMessage）
- 构造 tg.InputPeer
- 处理 access_hash
- 解析 gotd 原始类型
- 映射为中立 DTO
- 将 tgerr 映射为中立错误码
- 不向上层暴露 gotd 类型

## 哪些包允许依赖 gotd

| 包 | 允许的 gotd 依赖 | 原因 |
|---|---|---|
| `internal/telegramclient/gotd` | tg, tgerr, telegram, dcs | adapter 实现 |
| `internal/mtproto` | tg, tgerr, telegram, auth, dcs | 登录/认证流程 |
| `internal/server` (proxy_helper.go) | dcs.DialFunc | 代理拨号函数类型 |

## 哪些包禁止依赖 gotd

| 包 | 禁止的依赖 |
|---|---|
| `internal/chat` | tg, tgerr, telegram, dcs |
| `internal/model` | 所有 gotd 包 |
| `internal/telegramclient` (根包) | 所有 gotd 包 |
| `frontend/types` | 所有 gotd 包 |

## ChatService 如何依赖 adapter

```go
type ChatService struct {
    db      *gorm.DB
    key     []byte
    adapter telegramclient.ClientAdapter  // 通过接口注入
    logger  *slog.Logger
}

func NewChatService(db *gorm.DB, key []byte, adapter telegramclient.ClientAdapter, logger *slog.Logger) *ChatService
```

ChatService 从数据库获取账号凭据和 peer 缓存，然后通过 adapter 调用 Telegram。

## 切换 TDLib 时预期修改的文件

- `internal/telegramclient/tdlib/` — 新增 adapter 实现
- `internal/server/` 中的 composition root — 切换 adapter 注入
- `go.mod` — 添加 TDLib Go binding 依赖

## 切换 TDLib 时不应修改的文件

- `internal/chat/` 业务逻辑
- `internal/server/` HTTP handler（除 composition root）
- `internal/model/` 数据模型
- `frontend/` API types 和 UI
- `internal/telegramclient/types.go` 中立 DTO
- `internal/telegramclient/errors.go` 中立错误码
