package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"

	"github.com/gin-gonic/gin"
)

// handleGetSettings 处理 GET /settings - 系统设置页面。
func (s *Server) handleGetSettings(c *gin.Context) {
	data := s.newAuthViewData(c, "settings")

	// 获取默认 API Key
	credSvc := credential.NewService(s.db, s.key)
	defaultCred, err := credSvc.GetDefault()
	if err == nil && defaultCred != nil {
		data["DefaultCredential"] = defaultCred
	}

	// 获取所有凭据列表（用于高级区域）
	allCreds, _ := credSvc.List()
	data["CredentialList"] = allCreds

	// 获取代理设置
	proxySettings := s.getProxySettings()
	data["Proxy"] = proxySettings

	// 获取更新信息
	data["UpdateInfo"] = s.handleGetSettingsUpdate(c)

	// 处理查询参数中的消息
	if success := c.Query("success"); success != "" {
		data["Success"] = success
	}
	if errMsg := c.Query("error"); errMsg != "" {
		data["Error"] = errMsg
	}

	c.HTML(http.StatusOK, "settings.html", data)
}

// getProxySettings 读取代理设置。
func (s *Server) getProxySettings() map[string]string {
	settings := map[string]string{
		"proxy_type":     "none",
		"proxy_host":     "",
		"proxy_port":     "",
		"proxy_username": "",
		"proxy_timeout":  "",
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
		case "proxy_timeout":
			settings["proxy_timeout"] = setting.Value
		case "proxy_remark":
			settings["proxy_remark"] = setting.Value
		}
	}

	// 检查是否有密码
	var pwdSetting model.SystemSetting
	if err := s.db.Where("key = ?", "proxy_password").First(&pwdSetting).Error; err == nil {
		settings["has_password"] = "true"
	}

	return settings
}

// handlePostSettingsAPIKey 处理 POST /settings/api-key - 保存 API Key 配置。
func (s *Server) handlePostSettingsAPIKey(c *gin.Context) {
	adminID := auth.GetAdminID(c)
	if adminID == 0 {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	displayName := c.PostForm("api_display_name")
	apiIDStr := c.PostForm("api_id")
	apiHash := c.PostForm("api_hash")

	credSvc := credential.NewService(s.db, s.key)

	// 获取当前默认凭据
	defaultCred, _ := credSvc.GetDefault()

	if defaultCred == nil {
		// 没有默认凭据，创建新的
		if apiIDStr == "" || apiHash == "" {
			s.redirectSettingsWithError(c, "API ID 和 API Hash 不能为空")
			return
		}

		if displayName == "" {
			displayName = "Default API"
		}

		_, err := credSvc.Create(credential.CreateInput{
			DisplayName: displayName,
			APIID:       apiIDStr,
			APIHash:     apiHash,
			Status:      "enabled",
			RiskPolicy:  "disabled",
		})
		if err != nil {
			s.redirectSettingsWithError(c, "创建 API Key 失败: "+err.Error())
			return
		}

		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", adminID),
			Action:       "settings.api_key_created",
			ResourceType: "settings",
			ResourceID:   "api_key",
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "创建默认 API Key",
		})

		s.redirectSettingsWithSuccess(c, "API Key 配置已保存")
		return
	}

	// 已有默认凭据，更新
	if displayName != "" && displayName != defaultCred.DisplayName {
		_, err := credSvc.Update(defaultCred.ID, credential.UpdateInput{
			DisplayName: displayName,
			APIID:       fmt.Sprintf("%d", defaultCred.APIID),
			APIHash:     apiHash,
			Status:      string(defaultCred.Status),
			RiskPolicy:  string(defaultCred.RiskPolicy),
		})
		if err != nil {
			s.redirectSettingsWithError(c, "更新 API Key 失败: "+err.Error())
			return
		}
	} else if apiHash != "" {
		// 只更新 hash
		_, err := credSvc.Update(defaultCred.ID, credential.UpdateInput{
			DisplayName: defaultCred.DisplayName,
			APIID:       fmt.Sprintf("%d", defaultCred.APIID),
			APIHash:     apiHash,
			Status:      string(defaultCred.Status),
			RiskPolicy:  string(defaultCred.RiskPolicy),
		})
		if err != nil {
			s.redirectSettingsWithError(c, "更新 API Key 失败: "+err.Error())
			return
		}
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", adminID),
		Action:       "settings.api_key_updated",
		ResourceType: "settings",
		ResourceID:   "api_key",
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "更新 API Key 配置",
	})

	s.redirectSettingsWithSuccess(c, "API Key 配置已保存")
}

// handlePostSettingsProxy 处理 POST /settings/proxy - 保存代理配置。
func (s *Server) handlePostSettingsProxy(c *gin.Context) {
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
	proxyTimeout := c.PostForm("proxy_timeout")
	proxyRemark := c.PostForm("proxy_remark")

	// 校验
	if proxyType != "none" && proxyType != "https" && proxyType != "socks5" {
		s.redirectSettingsWithError(c, "无效的代理类型")
		return
	}

	if proxyType != "none" {
		if proxyHost == "" {
			s.redirectSettingsWithError(c, "代理主机不能为空")
			return
		}

		port, err := strconv.Atoi(proxyPort)
		if err != nil || port < 1 || port > 65535 {
			s.redirectSettingsWithError(c, "无效的代理端口")
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
	saveSetting("proxy_timeout", proxyTimeout, false)
	saveSetting("proxy_remark", proxyRemark, false)

	// 只有提供了新密码才更新
	if proxyPassword != "" {
		encryptedPassword, err := crypto.EncryptString(s.key, proxyPassword, []byte("atria:proxy:v1"))
		if err != nil {
			slog.Error("加密代理密码失败", "error", err)
		} else {
			saveSetting("proxy_password", encryptedPassword, true)
		}
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", adminID),
		Action:       "settings.proxy_updated",
		ResourceType: "settings",
		ResourceID:   "proxy",
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "代理设置已更新",
	})

	s.redirectSettingsWithSuccess(c, "代理配置已保存")
}

// handlePostAccountSelect 处理 POST /accounts/select - 切换当前账号。
func (s *Server) handlePostAccountSelect(c *gin.Context) {
	adminID := auth.GetAdminID(c)
	if adminID == 0 {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	accountIDStr := c.PostForm("account_id")
	if accountIDStr == "" {
		c.Redirect(http.StatusFound, "/accounts")
		return
	}

	accountID, err := strconv.ParseUint(accountIDStr, 10, 32)
	if err != nil {
		c.Redirect(http.StatusFound, "/accounts")
		return
	}

	// 验证账号存在
	var account model.TelegramAccount
	if err := s.db.First(&account, uint(accountID)).Error; err != nil {
		c.Redirect(http.StatusFound, "/accounts")
		return
	}

	// 保存到 session
	s.setSelectedAccountID(c, uint(accountID))

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", adminID),
		Action:       "account.session_selected",
		ResourceType: "account",
		ResourceID:   accountIDStr,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      fmt.Sprintf("切换当前账号 ID=%d", accountID),
	})

	// 重定向回来源页
	referer := c.GetHeader("Referer")
	if referer != "" {
		c.Redirect(http.StatusFound, referer)
	} else {
		c.Redirect(http.StatusFound, "/accounts")
	}
}

// setSelectedAccountID 设置当前选中的账号 ID 到 session。
func (s *Server) setSelectedAccountID(c *gin.Context, accountID uint) {
	token, _ := c.Cookie(s.cfg.CookieName)
	if token == "" {
		return
	}

	claims, err := auth.DecodeSessionToken(s.key, token)
	if err != nil {
		return
	}

	claims.SelectedAccountID = accountID

	newToken, err := auth.EncodeSessionToken(s.key, claims)
	if err != nil {
		slog.Error("编码 session token 失败", "error", err)
		return
	}

	c.SetCookie(
		s.cfg.CookieName,
		newToken,
		int(s.cfg.SessionTTL.Seconds()),
		"/",
		"",
		s.cfg.CookieSecure,
		true,
	)
}

// redirectSettingsWithError 重定向到设置页并显示错误。
func (s *Server) redirectSettingsWithError(c *gin.Context, msg string) {
	c.Redirect(http.StatusFound, "/settings?error="+msg)
}

// redirectSettingsWithSuccess 重定向到设置页并显示成功。
func (s *Server) redirectSettingsWithSuccess(c *gin.Context, msg string) {
	c.Redirect(http.StatusFound, "/settings?success="+msg)
}
