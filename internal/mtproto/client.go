package mtproto

import (
	"context"
	"time"
)

// LoginState 表示登录流程状态。
type LoginState string

const (
	LoginStateWaitingPhone    LoginState = "waiting_phone"
	LoginStateCodeSent        LoginState = "code_sent"
	LoginStateWaitingPassword LoginState = "waiting_password"
	LoginStateAuthorized      LoginState = "authorized"
	LoginStateFailed          LoginState = "failed"
	LoginStateExpired         LoginState = "expired"
)

// StartLoginRequest 是开始登录的请求。
type StartLoginRequest struct {
	APICredentialID uint
	APIID           int
	APIHash         string
	Phone           string
	FlowID          string // 登录流程 ID，用于关联临时 Session
}

// SubmitCodeRequest 是提交验证码的请求。
type SubmitCodeRequest struct {
	FlowID  string
	Code    string
	APIID   int
	APIHash string
}

// SubmitPasswordRequest 是提交 2FA 密码的请求。
type SubmitPasswordRequest struct {
	FlowID   string
	Password string
	APIID    int
	APIHash  string
}

// SyncProfileRequest 是同步账号资料的请求。
type SyncProfileRequest struct {
	APICredentialID uint
	APIID           int
	APIHash         string
	AccountID       uint
	SessionFilePath string
}

// CheckSessionRequest 是检查 Session 状态的请求。
type CheckSessionRequest struct {
	APICredentialID uint
	APIID           int
	APIHash         string
	AccountID       uint
	SessionFilePath string
}

// LogoutRequest 是登出请求。
type LogoutRequest struct {
	APICredentialID uint
	APIID           int
	APIHash         string
	AccountID       uint
	SessionFilePath string
}

// LoginStep 表示登录流程的一个步骤结果。
type LoginStep struct {
	FlowID      string
	State       LoginState
	PhoneHint   string
	Message     string
	Account     *AccountProfile
	SessionData []byte // 登录成功后的 gotd session 数据，用于保存为正式 session
}

// AccountProfile 表示 Telegram 账号资料。
type AccountProfile struct {
	UserID       int64
	Phone        string
	Username     string
	FirstName    string
	LastName     string
	IsPremium    bool
	IsRestricted bool
	IsScam       bool
	IsFake       bool
}

// SessionStatus 表示 Session 状态检查结果。
type SessionStatus struct {
	Valid     bool
	Status    string // active, invalid, logged_out, error
	Message   string
	CheckedAt time.Time
}

// Client 定义 MTProto 客户端接口。
// 业务层只能依赖此接口，不得直接依赖 gotd/td 具体类型。
type Client interface {
	// StartLogin 开始登录流程，发送验证码。
	StartLogin(ctx context.Context, req StartLoginRequest) (*LoginStep, error)

	// SubmitCode 提交验证码。
	SubmitCode(ctx context.Context, req SubmitCodeRequest) (*LoginStep, error)

	// SubmitPassword 提交 2FA 密码。
	SubmitPassword(ctx context.Context, req SubmitPasswordRequest) (*LoginStep, error)

	// SyncProfile 同步账号资料（基于已保存的 Session）。
	SyncProfile(ctx context.Context, req SyncProfileRequest) (*AccountProfile, error)

	// CheckSession 检查 Session 是否有效。
	CheckSession(ctx context.Context, req CheckSessionRequest) (*SessionStatus, error)

	// Logout 从 Telegram 服务器登出。
	Logout(ctx context.Context, req LogoutRequest) error
}
