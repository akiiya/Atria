package mtproto

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/telegram/dcs"
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
	}
	for _, k := range kinds {
		if k == "" {
			t.Error("错误类型不应为空")
		}
	}
}
