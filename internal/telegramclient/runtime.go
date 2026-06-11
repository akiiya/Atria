package telegramclient

import "time"

// RuntimeManager 定义 Telegram 运行时管理器接口。
// 用于管理长连接、实时更新推送等。
// 本轮只定义接口，不要求完整实现。
type RuntimeManager interface {
	// StartAccount 启动指定账号的运行时连接。
	StartAccount(accountID uint) error

	// StopAccount 停止指定账号的运行时连接。
	StopAccount(accountID uint) error

	// Status 获取指定账号的运行时状态。
	Status(accountID uint) RuntimeStatus

	// Subscribe 订阅指定账号的更新事件。
	Subscribe(accountID uint, sink UpdateSink) (Subscription, error)
}

// RuntimeStatus 表示运行时状态。
type RuntimeStatus struct {
	AccountID  uint         `json:"account_id"`
	State      RuntimeState `json:"state"`
	LastSyncAt *time.Time   `json:"last_sync_at,omitempty"`
	LastError  string       `json:"last_error,omitempty"`
}

// RuntimeState 表示运行时连接状态。
type RuntimeState string

const (
	RuntimeStateStopped    RuntimeState = "stopped"
	RuntimeStateConnecting RuntimeState = "connecting"
	RuntimeStateSyncing    RuntimeState = "syncing"
	RuntimeStateLive       RuntimeState = "live"
	RuntimeStateDegraded   RuntimeState = "degraded"
	RuntimeStateOffline    RuntimeState = "offline"
)

// UpdateEvent 表示一个更新事件。
type UpdateEvent struct {
	EventID   string          `json:"event_id"`
	AccountID uint            `json:"account_id"`
	Type      UpdateEventType `json:"type"`
	PeerRef   string          `json:"peer_ref,omitempty"`
	Payload   interface{}     `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// UpdateEventType 表示更新事件类型。
type UpdateEventType string

const (
	EventSyncStarted         UpdateEventType = "sync.started"
	EventSyncProgress        UpdateEventType = "sync.progress"
	EventSyncDone            UpdateEventType = "sync.done"
	EventSyncFailed          UpdateEventType = "sync.failed"
	EventDialogUpserted      UpdateEventType = "dialog.upserted"
	EventDialogUnreadUpdated UpdateEventType = "dialog.unread_updated"
	EventMessageNew          UpdateEventType = "message.new"
	EventMessageEdited       UpdateEventType = "message.edited"
	EventMessageDeleted      UpdateEventType = "message.deleted"
	EventMessageRead         UpdateEventType = "message.read"
	EventAccountConnected    UpdateEventType = "account.connected"
	EventAccountReconnecting UpdateEventType = "account.reconnecting"
	EventAccountDisconnected UpdateEventType = "account.disconnected"
)

// UpdateSink 是更新事件的接收端。
type UpdateSink interface {
	// Send 发送一个更新事件。实现方应保证非阻塞。
	Send(event UpdateEvent) error
}

// Subscription 表示一个更新订阅。
type Subscription interface {
	// Close 取消订阅。
	Close() error
}
