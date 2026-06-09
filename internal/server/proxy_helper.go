package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/gotd/td/telegram/dcs"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/network"

	"gorm.io/gorm"
)

// ProxyDialerFunc 是代理拨号函数类型。
type ProxyDialerFunc = dcs.DialFunc

// BuildProxyDialerFromDB 从数据库读取代理配置，返回 gotd 兼容的 DialFunc。
// 此函数可被登录流程和聊天流程复用。
// 如果代理未启用或类型为 none，返回 nil（直连）。
// 如果代理配置不完整或解密失败，返回 error。
func BuildProxyDialerFromDB(db *gorm.DB, key []byte) (ProxyDialerFunc, error) {
	var settings []model.SystemSetting
	db.Where("key IN ?", []string{"proxy_enabled", "proxy_type", "proxy_host", "proxy_port", "proxy_username", "proxy_timeout"}).Find(&settings)

	settingMap := make(map[string]string, len(settings))
	for _, st := range settings {
		settingMap[st.Key] = st.Value
	}

	// 检查代理是否启用
	if settingMap["proxy_enabled"] != "true" && settingMap["proxy_type"] == "none" {
		return nil, nil
	}

	proxyType := settingMap["proxy_type"]
	if proxyType == "none" || proxyType == "" {
		return nil, nil
	}

	host := settingMap["proxy_host"]
	portStr := settingMap["proxy_port"]
	if host == "" || portStr == "" {
		return nil, fmt.Errorf("代理配置不完整，请检查代理类型、主机和端口")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("代理端口无效: %s", portStr)
	}

	timeout := 30 * time.Second
	if t := settingMap["proxy_timeout"]; t != "" {
		if secs, err := strconv.Atoi(t); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}

	username := settingMap["proxy_username"]

	// 读取代理密码（加密存储）
	// proxy_password 记录缺失时视为空密码（合法）
	// proxy_password 存在但解密失败时返回错误，不得静默降级
	password := ""
	var pwdSetting model.SystemSetting
	err = db.Where("key = ?", "proxy_password").First(&pwdSetting).Error
	if err == gorm.ErrRecordNotFound {
		// 记录缺失，视为空密码，不打印 record not found 噪音日志
	} else if err != nil {
		slog.Error("查询代理密码失败", "error", err)
		return nil, fmt.Errorf("查询代理密码失败")
	} else if pwdSetting.Value != "" {
		decrypted, err := crypto.DecryptString(key, pwdSetting.Value, []byte("atria:proxy:v1"))
		if err != nil {
			slog.Error("解密代理密码失败，请检查代理配置", "error", err)
			return nil, fmt.Errorf("代理密码配置错误，请重新配置代理")
		}
		password = decrypted
	}

	config := network.ProxyConfig{
		Type:     network.ProxyType(proxyType),
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Timeout:  timeout,
	}

	dialer := network.NewDialer(config)
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}, nil
}
