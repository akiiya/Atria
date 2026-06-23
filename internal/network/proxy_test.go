package network

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestNewDialer_None_ReturnsDirect(t *testing.T) {
	config := ProxyConfig{
		Type:    ProxyTypeNone,
		Timeout: 5 * time.Second,
	}

	dialer := NewDialer(config)
	if dialer == nil {
		t.Fatal("NewDialer 不应返回 nil")
	}

	// 验证是直连拨号器
	if _, ok := dialer.(*directDialer); !ok {
		t.Error("none 类型应返回 directDialer")
	}
}

func TestNewDialer_EmptyType_ReturnsDirect(t *testing.T) {
	config := ProxyConfig{
		Type:    "",
		Timeout: 5 * time.Second,
	}

	dialer := NewDialer(config)
	if dialer == nil {
		t.Fatal("NewDialer 不应返回 nil")
	}

	if _, ok := dialer.(*directDialer); !ok {
		t.Error("空类型应返回 directDialer")
	}
}

func TestNewDialer_SOCKS5_ReturnsSocks5Dialer(t *testing.T) {
	config := ProxyConfig{
		Type:     ProxyTypeSOCKS5,
		Host:     "127.0.0.1",
		Port:     1080,
		Username: "user",
		Password: "pass",
		Timeout:  5 * time.Second,
	}

	dialer := NewDialer(config)
	if dialer == nil {
		t.Fatal("NewDialer 不应返回 nil")
	}

	if _, ok := dialer.(*socks5Dialer); !ok {
		t.Error("socks5 类型应返回 socks5Dialer")
	}
}

func TestNewDialer_HTTPS_ReturnsHTTPSConnectDialer(t *testing.T) {
	config := ProxyConfig{
		Type:     ProxyTypeHTTPS,
		Host:     "127.0.0.1",
		Port:     8080,
		Username: "user",
		Password: "pass",
		Timeout:  5 * time.Second,
	}

	dialer := NewDialer(config)
	if dialer == nil {
		t.Fatal("NewDialer 不应返回 nil")
	}

	httpsDialer, ok := dialer.(*httpsConnectDialer)
	if !ok {
		t.Error("https 类型应返回 httpsConnectDialer")
		return
	}

	if httpsDialer.proxyHost != "127.0.0.1" {
		t.Errorf("proxyHost 应为 127.0.0.1，实际=%s", httpsDialer.proxyHost)
	}
	if httpsDialer.proxyPort != 8080 {
		t.Errorf("proxyPort 应为 8080，实际=%d", httpsDialer.proxyPort)
	}
	if httpsDialer.username != "user" {
		t.Errorf("username 应为 user，实际=%s", httpsDialer.username)
	}
}

func TestNewDialer_HTTPS_DefaultPort(t *testing.T) {
	config := ProxyConfig{
		Type: ProxyTypeHTTPS,
		Host: "proxy.example.com",
		Port: 0, // 使用默认端口
	}

	dialer := NewDialer(config)
	httpsDialer, ok := dialer.(*httpsConnectDialer)
	if !ok {
		t.Fatal("应返回 httpsConnectDialer")
	}

	if httpsDialer.proxyPort != 443 {
		t.Errorf("默认端口应为 443，实际=%d", httpsDialer.proxyPort)
	}
}

func TestDirectDialer_DialContext_Timeout(t *testing.T) {
	dialer := &directDialer{timeout: 100 * time.Millisecond}

	// 尝试连接一个不可达的地址（应该超时）
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := dialer.DialContext(ctx, "tcp", "192.0.2.1:12345") // RFC 5737 测试地址
	if err == nil {
		t.Error("连接不可达地址应该失败")
	}
}

func TestHTTPSConnectDialer_DialContext_ErrorDoesNotLeakPassword(t *testing.T) {
	dialer := &httpsConnectDialer{
		proxyHost: "127.0.0.1",
		proxyPort: 1, // 不可达端口
		username:  "testuser",
		password:  "secretpassword123",
		timeout:   100 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := dialer.DialContext(ctx, "tcp", "target:443")
	if err == nil {
		t.Error("连接应该失败")
	}

	// 错误信息不应包含密码
	errMsg := err.Error()
	if contains(errMsg, "secretpassword123") {
		t.Error("错误信息不应包含密码")
	}
}

func TestSOCKS5Dialer_DialContext_ErrorDoesNotLeakPassword(t *testing.T) {
	// 创建一个 SOCKS5 dialer 指向不可达地址
	config := ProxyConfig{
		Type:     ProxyTypeSOCKS5,
		Host:     "127.0.0.1",
		Port:     1, // 不可达端口
		Username: "testuser",
		Password: "secretpassword123",
		Timeout:  100 * time.Millisecond,
	}

	dialer := NewDialer(config)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := dialer.DialContext(ctx, "tcp", "target:443")
	if err == nil {
		t.Error("连接应该失败")
	}

	// 错误信息不应包含密码
	errMsg := err.Error()
	if contains(errMsg, "secretpassword123") {
		t.Error("错误信息不应包含密码")
	}
}

func TestProxyConfig_Fields(t *testing.T) {
	config := ProxyConfig{
		Type:     ProxyTypeHTTPS,
		Host:     "proxy.example.com",
		Port:     8080,
		Username: "user",
		Password: "pass",
		Timeout:  30 * time.Second,
	}

	if config.Type != ProxyTypeHTTPS {
		t.Errorf("Type 应为 https，实际=%s", config.Type)
	}
	if config.Host != "proxy.example.com" {
		t.Errorf("Host 应为 proxy.example.com，实际=%s", config.Host)
	}
	if config.Port != 8080 {
		t.Errorf("Port 应为 8080，实际=%d", config.Port)
	}
	if config.Username != "user" {
		t.Errorf("Username 应为 user，实际=%s", config.Username)
	}
	if config.Password != "pass" {
		t.Errorf("Password 应为 pass，实际=%s", config.Password)
	}
}

func TestProxyType_Constants(t *testing.T) {
	if ProxyTypeNone != "none" {
		t.Errorf("ProxyTypeNone 应为 none，实际=%s", ProxyTypeNone)
	}
	if ProxyTypeHTTPS != "https" {
		t.Errorf("ProxyTypeHTTPS 应为 https，实际=%s", ProxyTypeHTTPS)
	}
	if ProxyTypeSOCKS5 != "socks5" {
		t.Errorf("ProxyTypeSOCKS5 应为 socks5，实际=%s", ProxyTypeSOCKS5)
	}
}

func TestProxyType_APIProxy_Removed(t *testing.T) {
	// api_proxy 已移除，ProxyTypeAPIProxy 常量不再存在
	// 如果旧数据库残留 api_proxy，应由 BuildProxyDialerFromDB 处理为 legacy invalid
	config := ProxyConfig{
		Type:    ProxyType("api_proxy"),
		Timeout: 5 * time.Second,
	}

	dialer := NewDialer(config)
	if dialer == nil {
		t.Fatal("NewDialer 不应返回 nil")
	}

	// api_proxy 不是 socks5/https，应返回 directDialer（default case）
	if _, ok := dialer.(*directDialer); !ok {
		t.Error("未知类型应返回 directDialer")
	}
}

// contains 检查字符串是否包含子串。
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockDialer 用于测试的 mock 拨号器。
type mockDialer struct {
	conn net.Conn
	err  error
}

func (m *mockDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return m.conn, m.err
}
