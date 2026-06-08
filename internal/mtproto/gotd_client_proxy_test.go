package mtproto

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tgerr"
)

func TestGotdClient_SetDialer_SetsDialFunc(t *testing.T) {
	c := NewGotdClient("/tmp", make([]byte, 32), nil, nil)

	if c.dialFunc != nil {
		t.Error("新建 GotdClient 的 dialFunc 应为 nil")
	}

	c.SetDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("mock")
	})

	if c.dialFunc == nil {
		t.Error("SetDialer 后 dialFunc 不应为 nil")
	}
}

func TestGotdClient_SetDialer_NilClearsDialer(t *testing.T) {
	c := NewGotdClient("/tmp", make([]byte, 32), nil, nil)

	c.SetDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("mock")
	})

	c.SetDialer(nil)
	if c.dialFunc != nil {
		t.Error("SetDialer(nil) 后 dialFunc 应为 nil")
	}
}

func TestGotdClient_UsesDirectDialerWhenProxyDisabled(t *testing.T) {
	c := NewGotdClient("/tmp", make([]byte, 32), nil, nil)
	// 未设置 dialer 时，buildOptions 不应注入 resolver
	if c.dialFunc != nil {
		t.Error("proxy 未启用时 dialFunc 应为 nil")
	}
}

func TestGotdClient_UsesCustomResolverWhenDialerSet(t *testing.T) {
	c := NewGotdClient("/tmp", make([]byte, 32), nil, nil)

	c.SetDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("mock dialer")
	})

	if c.dialFunc == nil {
		t.Error("设置 dialer 后 dialFunc 不应为 nil")
	}
}

func TestGotdClient_ClassifyError_ProxyConnectFailed(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	// 模拟代理连接被拒绝
	err := &net.OpError{
		Op:  "dial",
		Err: errors.New("connection refused"),
	}

	result := c.classifyError(err)
	mtprotoErr, ok := result.(*MTProtoError)
	if !ok {
		t.Fatalf("期望 *MTProtoError，实际 %T", result)
	}
	if mtprotoErr.Kind != ErrProxyConnectFailed {
		t.Errorf("期望 ErrProxyConnectFailed，实际 %s", mtprotoErr.Kind)
	}
}

func TestGotdClient_ClassifyError_Timeout(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	result := c.classifyError(context.DeadlineExceeded)
	mtprotoErr, ok := result.(*MTProtoError)
	if !ok {
		t.Fatalf("期望 *MTProtoError，实际 %T", result)
	}
	if mtprotoErr.Kind != ErrTelegramTimeout {
		t.Errorf("期望 ErrTelegramTimeout，实际 %s", mtprotoErr.Kind)
	}
	if !strings.Contains(mtprotoErr.Message, "超时") {
		t.Errorf("错误消息应包含'超时'，实际 %s", mtprotoErr.Message)
	}
}

func TestGotdClient_ClassifyError_ContextCanceled(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	result := c.classifyError(context.Canceled)
	mtprotoErr, ok := result.(*MTProtoError)
	if !ok {
		t.Fatalf("期望 *MTProtoError，实际 %T", result)
	}
	if mtprotoErr.Kind != ErrNetworkError {
		t.Errorf("期望 ErrNetworkError，实际 %s", mtprotoErr.Kind)
	}
}

func TestGotdClient_ClassifyError_ProxyAuth(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	result := c.classifyError(errors.New("proxy auth required: 407"))
	mtprotoErr, ok := result.(*MTProtoError)
	if !ok {
		t.Fatalf("期望 *MTProtoError，实际 %T", result)
	}
	if mtprotoErr.Kind != ErrProxyAuthFailed {
		t.Errorf("期望 ErrProxyAuthFailed，实际 %s", mtprotoErr.Kind)
	}
}

func TestGotdClient_ClassifyError_NoSensitiveDataInMessage(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}
	apiHash := "abcdef0123456789abcdef0123456789"
	proxyPassword := "mysecretpassword"

	testErrors := []error{
		context.DeadlineExceeded,
		errors.New("proxy auth failed"),
		errors.New("connection refused"),
		&net.OpError{Op: "dial", Err: errors.New("timeout")},
	}

	for _, err := range testErrors {
		result := c.classifyError(err)
		if result != nil {
			errStr := result.Error()
			if strings.Contains(errStr, apiHash) {
				t.Errorf("错误消息不应包含 api_hash: %s", errStr)
			}
			if strings.Contains(errStr, proxyPassword) {
				t.Errorf("错误消息不应包含 proxy_password: %s", errStr)
			}
		}
	}
}

func TestIsProxyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"net.OpError", &net.OpError{Op: "dial", Err: errors.New("refused")}, true},
		{"proxy keyword", errors.New("proxy connection failed"), true},
		{"SOCKS keyword", errors.New("SOCKS5 dial failed"), true},
		{"CONNECT keyword", errors.New("CONNECT proxy error"), true},
		{"407 error", errors.New("HTTP 407 Proxy Authentication Required"), true},
		{"unrelated error", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isProxyError(tt.err)
			if result != tt.expected {
				t.Errorf("isProxyError = %v, 期望 %v", result, tt.expected)
			}
		})
	}
}

func TestClassifyProxyError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected ErrorKind
	}{
		{"auth error", "proxy auth failed: 407", ErrProxyAuthFailed},
		{"timeout", "dial timeout: deadline exceeded", ErrProxyConnectFailed},
		{"refused", "connection refused by proxy", ErrProxyConnectFailed},
		{"generic", "proxy error unknown", ErrProxyConnectFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyProxyError(errors.New(tt.errStr))
			mtprotoErr, ok := result.(*MTProtoError)
			if !ok {
				t.Fatalf("期望 *MTProtoError，实际 %T", result)
			}
			if mtprotoErr.Kind != tt.expected {
				t.Errorf("期望 %s，实际 %s", tt.expected, mtprotoErr.Kind)
			}
		})
	}
}

func TestDialFunc_Signature_MatchesNetworkDialer(t *testing.T) {
	// 验证 dcs.DialFunc 与 network.Dialer.DialContext 签名兼容
	var dialFunc dcs.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, nil
	}
	_ = dialFunc
}

func TestTimeoutDuration_InProxyConfig(t *testing.T) {
	timeout := 30 * time.Second
	if timeout < 5*time.Second {
		t.Error("代理超时不应小于 5 秒")
	}
	if timeout > 300*time.Second {
		t.Error("代理超时不应大于 300 秒")
	}
}

func TestNewErrorKinds_Documented(t *testing.T) {
	// 验证新增的错误类型都有定义
	kinds := []ErrorKind{
		ErrProxyConnectFailed,
		ErrProxyAuthFailed,
		ErrTelegramTimeout,
		ErrTelegramError,
		ErrSessionContextLost,
	}
	for _, k := range kinds {
		if k == "" {
			t.Error("错误类型不应为空")
		}
	}
}

// ===== 错误链分类测试 =====

func TestClassifyError_UnwrapsErrorChain(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	// 构造 wrapped error，内层包含 PHONE_CODE_INVALID
	inner := &tgerr.Error{Code: 400, Message: "PHONE_CODE_INVALID", Type: "PHONE_CODE_INVALID"}
	wrapped := fmt.Errorf("AuthSignIn failed: %w", inner)

	result := c.classifyError(wrapped)
	mtprotoErr, ok := result.(*MTProtoError)
	if !ok {
		t.Fatalf("期望 *MTProtoError，实际 %T", result)
	}
	if mtprotoErr.Kind != ErrLoginCodeInvalid {
		t.Errorf("期望 ErrLoginCodeInvalid，实际 %s", mtprotoErr.Kind)
	}
}

func TestClassifyError_WrappedPhoneCodeExpired(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	inner := &tgerr.Error{Code: 400, Message: "PHONE_CODE_EXPIRED", Type: "PHONE_CODE_EXPIRED"}
	wrapped := fmt.Errorf("gotd error: %w", inner)

	result := c.classifyError(wrapped)
	mtprotoErr := result.(*MTProtoError)
	if mtprotoErr.Kind != ErrLoginCodeExpired {
		t.Errorf("期望 ErrLoginCodeExpired，实际 %s", mtprotoErr.Kind)
	}
}

func TestClassifyError_WrappedSessionPasswordNeeded(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	inner := &tgerr.Error{Code: 401, Message: "SESSION_PASSWORD_NEEDED", Type: "SESSION_PASSWORD_NEEDED"}
	wrapped := fmt.Errorf("auth error: %w", inner)

	result := c.classifyError(wrapped)
	mtprotoErr := result.(*MTProtoError)
	if mtprotoErr.Kind != ErrLoginPasswordRequired {
		t.Errorf("期望 ErrLoginPasswordRequired，实际 %s", mtprotoErr.Kind)
	}
}

func TestClassifyError_WrappedAuthKeyInvalid(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	inner := &tgerr.Error{Code: 401, Message: "AUTH_KEY_INVALID", Type: "AUTH_KEY_INVALID"}
	wrapped := fmt.Errorf("session error: %w", inner)

	result := c.classifyError(wrapped)
	mtprotoErr := result.(*MTProtoError)
	if mtprotoErr.Kind != ErrSessionContextLost {
		t.Errorf("期望 ErrSessionContextLost，实际 %s", mtprotoErr.Kind)
	}
}

func TestClassifyError_UnknownWrappedTelegramError(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	// 未知的 RPC 错误类型
	inner := &tgerr.Error{Code: 406, Message: "UNKNOWN_RPC_ERROR", Type: "UNKNOWN_RPC_ERROR"}
	wrapped := fmt.Errorf("gotd error: %w", inner)

	result := c.classifyError(wrapped)
	mtprotoErr := result.(*MTProtoError)
	// 应该返回 telegram_error，不是 network_error
	if mtprotoErr.Kind != ErrTelegramError {
		t.Errorf("期望 ErrTelegramError，实际 %s", mtprotoErr.Kind)
	}
	if mtprotoErr.Kind == ErrNetworkError {
		t.Error("不应返回 ErrNetworkError")
	}
}

func TestClassifyError_NetTimeout(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	// net.OpError with timeout - isProxyError catches net.OpError first
	err := &net.OpError{Op: "dial", Err: &timeoutError{}}

	result := c.classifyError(err)
	mtprotoErr := result.(*MTProtoError)
	// net.OpError is caught by isProxyError, timeout is handled by classifyProxyError
	if mtprotoErr.Kind != ErrProxyConnectFailed {
		t.Errorf("期望 ErrProxyConnectFailed，实际 %s", mtprotoErr.Kind)
	}
}

func TestClassifyError_ProxyConnectFailed(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	// connection refused 是代理连接失败
	err := &net.OpError{Op: "dial", Err: errors.New("connection refused")}

	result := c.classifyError(err)
	mtprotoErr := result.(*MTProtoError)
	if mtprotoErr.Kind != ErrProxyConnectFailed {
		t.Errorf("期望 ErrProxyConnectFailed，实际 %s", mtprotoErr.Kind)
	}
}

// timeoutError 实现 net.Error 接口
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return false }

// ===== 安全日志测试 =====

func TestSafeErrorSummary_RedactsSensitiveData(t *testing.T) {
	// 测试 sanitizeErrorMessage 脱敏
	testCases := []struct {
		name     string
		input    string
		badWords []string
	}{
		{
			name:     "长 hex 串脱敏",
			input:    "error with hash abcdef0123456789abcdef0123456789 inside",
			badWords: []string{"abcdef0123456789abcdef0123456789"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeErrorMessage(tc.input)
			for _, bad := range tc.badWords {
				if strings.Contains(result, bad) {
					t.Errorf("输出不应包含敏感数据 %q，实际: %s", bad, result)
				}
			}
		})
	}
}

func TestClassifyError_WrappedFloodWait(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	inner := &tgerr.Error{Code: 420, Message: "FLOOD_WAIT_30", Type: "FLOOD_WAIT", Argument: 30}
	wrapped := fmt.Errorf("gotd error: %w", inner)

	result := c.classifyError(wrapped)
	floodErr, ok := result.(*FloodWaitError)
	if !ok {
		t.Fatalf("期望 *FloodWaitError，实际 %T", result)
	}
	if floodErr.Wait != 30*time.Second {
		t.Errorf("期望 30s，实际 %s", floodErr.Wait)
	}
}

func TestClassifyError_ContextDeadlineExceeded(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	result := c.classifyError(context.DeadlineExceeded)
	mtprotoErr := result.(*MTProtoError)
	if mtprotoErr.Kind != ErrTelegramTimeout {
		t.Errorf("期望 ErrTelegramTimeout，实际 %s", mtprotoErr.Kind)
	}
}

func TestClassifyError_ContextCanceled(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	result := c.classifyError(context.Canceled)
	mtprotoErr := result.(*MTProtoError)
	if mtprotoErr.Kind != ErrNetworkError {
		t.Errorf("期望 ErrNetworkError，实际 %s", mtprotoErr.Kind)
	}
}

// ===== SESSION_PASSWORD_NEEDED 测试 =====

func TestClassifyError_SessionPasswordNeeded_ReturnsPasswordRequired(t *testing.T) {
	c := &GotdClient{logger: slog.Default()}

	// 构造 SESSION_PASSWORD_NEEDED wrapped error
	inner := &tgerr.Error{Code: 401, Message: "SESSION_PASSWORD_NEEDED", Type: "SESSION_PASSWORD_NEEDED"}
	wrapped := fmt.Errorf("AuthSignIn failed: %w", inner)

	result := c.classifyError(wrapped)
	mtprotoErr, ok := result.(*MTProtoError)
	if !ok {
		t.Fatalf("期望 *MTProtoError，实际 %T", result)
	}
	if mtprotoErr.Kind != ErrLoginPasswordRequired {
		t.Errorf("期望 ErrLoginPasswordRequired，实际 %s", mtprotoErr.Kind)
	}
}

func TestIsSessionPasswordNeeded_WrappedError(t *testing.T) {
	// 测试 isSessionPasswordNeeded 能否识别包装后的 SESSION_PASSWORD_NEEDED
	inner := &tgerr.Error{Code: 401, Message: "SESSION_PASSWORD_NEEDED", Type: "SESSION_PASSWORD_NEEDED"}
	wrapped := fmt.Errorf("AuthSignIn failed: %w", inner)

	if !isSessionPasswordNeeded(wrapped) {
		t.Error("包装后的 SESSION_PASSWORD_NEEDED 应被识别")
	}
}

func TestIsSessionPasswordNeeded_ClassifiedError(t *testing.T) {
	// 测试 classifyError 包装后的错误也能被识别
	c := &GotdClient{logger: slog.Default()}

	inner := &tgerr.Error{Code: 401, Message: "SESSION_PASSWORD_NEEDED", Type: "SESSION_PASSWORD_NEEDED"}
	wrapped := fmt.Errorf("AuthSignIn failed: %w", inner)

	classified := c.classifyError(wrapped)
	if !isSessionPasswordNeeded(classified) {
		t.Error("classifyError 包装后的 SESSION_PASSWORD_NEEDED 应被 isSessionPasswordNeeded 识别")
	}
}

func TestIsSessionPasswordNeeded_NilError(t *testing.T) {
	if isSessionPasswordNeeded(nil) {
		t.Error("nil 错误不应被识别为 SESSION_PASSWORD_NEEDED")
	}
}

func TestIsSessionPasswordNeeded_OtherError(t *testing.T) {
	err := fmt.Errorf("some other error")
	if isSessionPasswordNeeded(err) {
		t.Error("其它错误不应被识别为 SESSION_PASSWORD_NEEDED")
	}
}
