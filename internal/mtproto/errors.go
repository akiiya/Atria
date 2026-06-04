// Package mtproto 提供 MTProto 客户端抽象和 Telegram 账号接入框架。
package mtproto

import (
	"fmt"
	"time"
)

// ErrorKind 表示 MTProto 错误类型。
type ErrorKind string

const (
	ErrUnauthorized          ErrorKind = "unauthorized"
	ErrSessionInvalid        ErrorKind = "session_invalid"
	ErrLoginCodeRequired     ErrorKind = "login_code_required"
	ErrLoginPasswordRequired ErrorKind = "login_password_required"
	ErrLoginCodeInvalid      ErrorKind = "login_code_invalid"
	ErrLoginCodeExpired      ErrorKind = "login_code_expired"
	ErrLoginPasswordInvalid  ErrorKind = "login_password_invalid"
	ErrLoginExpired          ErrorKind = "login_expired"
	ErrFloodWait             ErrorKind = "flood_wait"
	ErrRateLimited           ErrorKind = "rate_limited"
	ErrCredentialDisabled    ErrorKind = "credential_disabled"
	ErrNotImplemented        ErrorKind = "not_implemented"
	ErrInvalidPhone          ErrorKind = "invalid_phone"
	ErrInvalidCode           ErrorKind = "invalid_code"
	ErrInvalidPassword       ErrorKind = "invalid_password"
	ErrNetworkError          ErrorKind = "network_error"
	ErrInternalError         ErrorKind = "internal_error"
)

// MTProtoError 表示 MTProto 操作错误。
type MTProtoError struct {
	Kind    ErrorKind
	Message string
	Err     error
}

func (e *MTProtoError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Kind, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Kind, e.Message)
}

func (e *MTProtoError) Unwrap() error {
	return e.Err
}

// FloodWaitError 表示 Telegram FLOOD_WAIT 错误。
// 当遇到此错误时，必须等待指定时间后才能重试。
// 禁止盲目重试，必须遵循等待时间。
type FloodWaitError struct {
	Wait    time.Duration
	Message string
}

func (e *FloodWaitError) Error() string {
	return fmt.Sprintf("FLOOD_WAIT: 需等待 %s 后重试: %s", e.Wait, e.Message)
}

// ClassifyError 将错误分类为 ErrorKind。
// 本轮只做基本分类，后续真实接入时需要解析 gotd/td 错误。
func ClassifyError(err error) ErrorKind {
	if err == nil {
		return ""
	}

	if _, ok := err.(*FloodWaitError); ok {
		return ErrFloodWait
	}

	if mtprotoErr, ok := err.(*MTProtoError); ok {
		return mtprotoErr.Kind
	}

	return ErrNetworkError
}
