package telegramclient

import "fmt"

// ErrorCode 是中立的错误码，不依赖任何 Telegram 库的错误类型。
// 上层业务只判断 ErrorCode，不解析 gotd tgerr 或 TDLib 错误。
type ErrorCode string

const (
	ErrorCodeNoCurrentAccount   ErrorCode = "no_current_account"
	ErrorCodeSessionInvalid     ErrorCode = "session_invalid"
	ErrorCodePeerInvalid        ErrorCode = "peer_invalid"
	ErrorCodePeerIncomplete     ErrorCode = "peer_incomplete"
	ErrorCodeProxyConnectFailed ErrorCode = "proxy_connect_failed"
	ErrorCodeProxyAuthFailed    ErrorCode = "proxy_auth_failed"
	ErrorCodeProxyConfigInvalid ErrorCode = "proxy_config_invalid"
	ErrorCodeTelegramTimeout    ErrorCode = "telegram_timeout"
	ErrorCodeAPIKeyInvalid      ErrorCode = "api_key_invalid"
	ErrorCodeFloodWait          ErrorCode = "flood_wait"
	ErrorCodeAuthRestart        ErrorCode = "auth_restart"
	ErrorCodeAccountDeactivated ErrorCode = "account_deactivated"
	ErrorCodeTelegramError      ErrorCode = "telegram_error"
	ErrorCodeNetworkError       ErrorCode = "network_error"
	ErrorCodeTextEmpty          ErrorCode = "text_empty"
	ErrorCodeTextTooLong        ErrorCode = "text_too_long"
)

// Error 是中立的 Telegram 客户端错误。
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Cause   error     `json:"-"`
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// NewError 创建一个新的中立错误。
func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

// NewErrorf 创建一个带格式化消息的中立错误。
func NewErrorf(code ErrorCode, format string, args ...interface{}) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// WrapError 包装一个底层错误为中立错误。
func WrapError(code ErrorCode, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}
