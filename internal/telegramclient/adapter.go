package telegramclient

import "context"

// DialFunc 是网络拨号函数类型。
// 用于代理支持，与 gotd/td/telegram/dcs.DialFunc 签名一致，
// 但不依赖 gotd 包。
type DialFunc func(ctx context.Context, network, addr string) (interface{ Close() error }, error)

// ClientAdapter 定义 Telegram 客户端适配器接口。
// 上层业务（ChatService）只依赖此接口，不直接使用 gotd 或 TDLib。
// 当前实现：gotd adapter。未来实现：TDLib adapter。
type ClientAdapter interface {
	// ListDialogs 获取会话列表。
	ListDialogs(ctx context.Context, req ListDialogsRequest) (DialogsPage, error)

	// GetRecentMessages 获取指定会话的最近消息。
	GetRecentMessages(ctx context.Context, req GetRecentMessagesRequest) (MessagesPage, error)

	// LoadOlderMessages 加载更早的消息（分页）。
	LoadOlderMessages(ctx context.Context, req LoadOlderMessagesRequest) (MessagesPage, error)

	// SendText 发送文本消息。
	SendText(ctx context.Context, req SendTextRequest) (SendResult, error)
}

// AdapterConfig 是适配器的通用配置。
type AdapterConfig struct {
	// SessionDir 是 session 文件目录。
	SessionDir string

	// Key 是加密密钥（用于 session 和 access_hash 解密）。
	Key []byte

	// APIID 是 Telegram API ID。
	APIID int

	// APIHash 是 Telegram API Hash（已解密）。
	APIHash string

	// SessionFilePath 是当前账号的 session 文件路径。
	SessionFilePath string

	// DialFunc 是代理拨号函数，nil 表示直连。
	DialFunc func(ctx context.Context, network, addr string) (interface{ Close() error }, error)
}
