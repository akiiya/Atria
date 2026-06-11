# TDLib Adapter（预留）

## 状态

本轮不实现 TDLib adapter。此目录仅用于记录未来设计方向。

## 设计约束

1. **必须实现 `telegramclient.ClientAdapter` 接口**。
   TDLib adapter 的输入输出必须使用 `internal/telegramclient` 中立 DTO。

2. **TDLib 原始类型不得泄漏到业务层**。
   `internal/chat`、`internal/server`、frontend API types 不得感知 TDLib 的存在。

3. **切换只替换 adapter 实现**。
   未来切 TDLib 时，只应修改：
   - `internal/telegramclient/tdlib/`（新增 adapter 实现）
   - `internal/server/` 中的 composition root（切换 adapter 注入）
   - `go.mod`（添加 TDLib Go binding 依赖）

   不应修改：
   - `internal/chat/` 业务逻辑
   - `internal/server/` HTTP handler（除 composition root）
   - `internal/model/` 数据模型
   - `frontend/` API types 和 UI

## TDLib 可能带来的部署影响

- **Native library**: TDLib 是 C++ 库，需要 CGO 或 pre-built binary
- **Windows 打包**: 需要提供 Windows DLL
- **跨平台发布**: 需要为每个目标平台编译 TDLib
- **数据目录**: TDLib 有本地数据库（tdlib.bin），需要管理存储路径
- **本地 TDLib store**: TDLib 维护自己的消息/对话缓存，与 SQLite 缓存可能重复

## 当前 gotd adapter 是过渡实现

gotd/td 是纯 Go 实现，部署简单，但：
- 社区维护活跃度不确定
- 某些 Telegram API 特性可能滞后
- 长连接/实时更新支持需要额外工作

TDLib 是 Telegram 官方推荐的客户端库，功能完整，但：
- 部署复杂度高
- 需要 CGO 或进程间通信

## 接口参考

### ClientAdapter

TDLib adapter 需要实现：

```go
type ClientAdapter interface {
    ListDialogs(ctx context.Context, req ListDialogsRequest) (DialogsPage, error)
    GetRecentMessages(ctx context.Context, req GetRecentMessagesRequest) (MessagesPage, error)
    LoadOlderMessages(ctx context.Context, req LoadOlderMessagesRequest) (MessagesPage, error)
    SendText(ctx context.Context, req SendTextRequest) (SendResult, error)
}
```

### RuntimeManager

TDLib runtime 需要实现：

```go
type RuntimeManager interface {
    StartAccount(accountID uint) error
    StopAccount(accountID uint) error
    Status(accountID uint) RuntimeStatus
    Subscribe(accountID uint, sink UpdateSink) (Subscription, error)
}
```

TDLib 的 update handler 需要将 TDLib 原始 update 映射为 `telegramclient.UpdateEvent`，与 gotd runtime 使用相同的事件类型。

### UpdateEvent 映射

TDLib 的 update 类型（td_api::UpdateNewMessage 等）需要映射为：
- `EventMessageNew` → `telegramclient.Message`
- `EventMessageEdited` → `telegramclient.Message`
- `EventMessageDeleted` → message IDs
- `EventDialogUpserted` → `telegramclient.Dialog`

所有映射在 `internal/telegramclient/tdlib/` 内部完成，不泄漏 TDLib 类型。

所有请求和返回类型定义在 `internal/telegramclient/types.go`。
