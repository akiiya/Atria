package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"

	"github.com/gin-gonic/gin"
)

// handleGetProxySettings 处理 GET /settings/proxy - 代理设置页面。
func (s *Server) handleGetProxySettings(c *gin.Context) {
	data := s.newAuthViewData(c, "settings")

	// 读取当前代理设置
	settings := map[string]string{
		"proxy_type":     "none",
		"proxy_host":     "",
		"proxy_port":     "",
		"proxy_username": "",
		"proxy_remark":   "",
	}

	var proxySettings []model.SystemSetting
	s.db.Where("key LIKE ?", "proxy_%").Find(&proxySettings)
	for _, setting := range proxySettings {
		switch setting.Key {
		case "proxy_type":
			settings["proxy_type"] = setting.Value
		case "proxy_host":
			settings["proxy_host"] = setting.Value
		case "proxy_port":
			settings["proxy_port"] = setting.Value
		case "proxy_username":
			settings["proxy_username"] = setting.Value
		case "proxy_remark":
			settings["proxy_remark"] = setting.Value
		}
	}

	// 检查是否有密码（不显示明文）
	var pwdSetting model.SystemSetting
	if err := s.db.Where("key = ?", "proxy_password").First(&pwdSetting).Error; err == nil {
		settings["has_password"] = "true"
	}

	data["Proxy"] = settings
	c.HTML(http.StatusOK, "proxy.html", data)
}

// handlePostProxySettings 处理 POST /settings/proxy - 保存代理设置。
func (s *Server) handlePostProxySettings(c *gin.Context) {
	adminID := auth.GetAdminID(c)
	if adminID == 0 {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	proxyType := c.PostForm("proxy_type")
	proxyHost := c.PostForm("proxy_host")
	proxyPort := c.PostForm("proxy_port")
	proxyUsername := c.PostForm("proxy_username")
	proxyPassword := c.PostForm("proxy_password")
	proxyRemark := c.PostForm("proxy_remark")

	// 校验
	if proxyType != "none" && proxyType != "https" && proxyType != "socks5" {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = "无效的代理类型"
		c.HTML(http.StatusOK, "proxy.html", data)
		return
	}

	if proxyType != "none" {
		if proxyHost == "" {
			data := s.newAuthViewData(c, "settings")
			data["Error"] = "代理主机不能为空"
			c.HTML(http.StatusOK, "proxy.html", data)
			return
		}

		port, err := strconv.Atoi(proxyPort)
		if err != nil || port < 1 || port > 65535 {
			data := s.newAuthViewData(c, "settings")
			data["Error"] = "无效的代理端口"
			c.HTML(http.StatusOK, "proxy.html", data)
			return
		}
	}

	// 保存设置
	saveSetting := func(key, value string, isSensitive bool) {
		setting := model.SystemSetting{
			Key:         key,
			Value:       value,
			ValueType:   "string",
			IsSensitive: isSensitive,
		}
		s.db.Where("key = ?", key).Assign(setting).FirstOrCreate(&model.SystemSetting{})
	}

	saveSetting("proxy_type", proxyType, false)
	saveSetting("proxy_host", proxyHost, false)
	saveSetting("proxy_port", proxyPort, false)
	saveSetting("proxy_username", proxyUsername, false)
	saveSetting("proxy_remark", proxyRemark, false)

	// 只有提供了新密码才更新
	if proxyPassword != "" {
		// 加密密码
		encryptedPassword, err := s.encryptSensitiveValue(proxyPassword)
		if err != nil {
			slog.Error("加密代理密码失败", "error", err)
		} else {
			saveSetting("proxy_password", encryptedPassword, true)
		}
	}

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      "0",
		Action:       "settings.proxy_updated",
		ResourceType: "settings",
		ResourceID:   "proxy",
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "代理设置已更新",
	})

	data := s.newAuthViewData(c, "settings")
	data["Success"] = "代理设置已保存"

	// 重新读取设置
	settings := map[string]string{
		"proxy_type":     proxyType,
		"proxy_host":     proxyHost,
		"proxy_port":     proxyPort,
		"proxy_username": proxyUsername,
		"proxy_remark":   proxyRemark,
	}
	if proxyPassword != "" {
		settings["has_password"] = "true"
	}
	data["Proxy"] = settings

	c.HTML(http.StatusOK, "proxy.html", data)
}

// encryptSensitiveValue 加密敏感值。
func (s *Server) encryptSensitiveValue(value string) (string, error) {
	return crypto.EncryptString(s.key, value, []byte("atria:proxy:v1"))
}
