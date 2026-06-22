// Package network 提供网络连接代理能力。
package network

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyType 表示代理类型。
type ProxyType string

const (
	ProxyTypeNone     ProxyType = "none"
	ProxyTypeHTTPS    ProxyType = "https"
	ProxyTypeSOCKS5   ProxyType = "socks5"
	ProxyTypeAPIProxy ProxyType = "api_proxy"
)

// ProxyConfig 代理配置。
type ProxyConfig struct {
	Type     ProxyType
	Host     string
	Port     int
	Username string
	Password string
	Timeout  time.Duration
}

// Dialer 是网络连接拨号接口。
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// directDialer 直连拨号器。
type directDialer struct {
	timeout time.Duration
}

func (d *directDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: d.timeout}
	return dialer.DialContext(ctx, network, address)
}

// socks5Dialer SOCKS5 代理拨号器。
type socks5Dialer struct {
	dialer proxy.Dialer
}

func (d *socks5Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// golang.org/x/net/proxy 不直接支持 DialContext，使用 Dial
	// 对于 SOCKS5，Dial 通常是安全的
	conn, err := d.dialer.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 代理连接失败: %w", err)
	}
	return conn, nil
}

// httpsConnectDialer HTTPS CONNECT 代理拨号器。
type httpsConnectDialer struct {
	proxyHost string
	proxyPort int
	username  string
	password  string
	timeout   time.Duration
}

func (d *httpsConnectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// 建立到代理服务器的 TCP 连接
	proxyAddr := fmt.Sprintf("%s:%d", d.proxyHost, d.proxyPort)
	dialer := &net.Dialer{Timeout: d.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("连接代理服务器失败: %w", err)
	}

	// 发送 CONNECT 请求
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}

	// 添加代理认证
	if d.username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(d.username + ":" + d.password))
		connectReq.Header.Set("Proxy-Authorization", "Basic "+auth)
	}

	// 写入 CONNECT 请求
	if err := connectReq.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("发送 CONNECT 请求失败: %w", err)
	}

	// 读取响应
	resp, err := http.ReadResponse(bufio.NewReader(conn), connectReq)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("读取代理响应失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("代理连接失败: HTTP %d", resp.StatusCode)
	}

	return conn, nil
}

// NewDialer 根据代理配置创建拨号器。
func NewDialer(config ProxyConfig) Dialer {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	switch config.Type {
	case ProxyTypeSOCKS5:
		// 创建 SOCKS5 拨号器
		addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
		var auth *proxy.Auth
		if config.Username != "" {
			auth = &proxy.Auth{
				User:     config.Username,
				Password: config.Password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
		if err != nil {
			// 如果创建失败，返回直连拨号器
			return &directDialer{timeout: timeout}
		}

		return &socks5Dialer{dialer: dialer}

	case ProxyTypeHTTPS:
		port := config.Port
		if port == 0 {
			port = 443
		}

		return &httpsConnectDialer{
			proxyHost: config.Host,
			proxyPort: port,
			username:  config.Username,
			password:  config.Password,
			timeout:   timeout,
		}

	default:
		// 直连
		return &directDialer{timeout: timeout}
	}
}
