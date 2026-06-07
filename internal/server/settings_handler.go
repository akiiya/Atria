package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/network"

	"github.com/gin-gonic/gin"
)

// SystemAPIKeyData 系统 API Key 展示数据。
type SystemAPIKeyData struct {
	DisplayName  string
	APIID        int32
	APIIDMasked  string
	APIHashHint  string
	Status       string
	UpdatedAt    string
	HasSystemKey bool
}

// maskAPIID 脱敏 API ID。
func maskAPIID(apiID int32) string {
	s := fmt.Sprintf("%d", apiID)
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

// handleGetSettings 处理 GET /settings - 系统设置页面。
func (s *Server) handleGetSettings(c *gin.Context) {
	data := s.newAuthViewData(c, "settings")

	// 获取系统 API Key（单例）
	credSvc := credential.NewService(s.db, s.key)
	systemKey, _ := credSvc.GetSystemAPIKey()
	if systemKey != nil {
		apiKeyData := SystemAPIKeyData{
			DisplayName:  systemKey.DisplayName,
			APIID:        systemKey.APIID,
			APIIDMasked:  maskAPIID(systemKey.APIID),
			APIHashHint:  systemKey.APIHashHint,
			Status:       string(systemKey.Status),
			UpdatedAt:    systemKey.UpdatedAt.Format("2006-01-02 15:04"),
			HasSystemKey: true,
		}
		data["SystemAPIKey"] = apiKeyData
	}

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

	// 获取当前系统 API Key
	systemKey, _ := credSvc.GetSystemAPIKey()

	if systemKey == nil {
		// 没有系统 API Key，创建新的
		if apiIDStr == "" || apiHash == "" {
			s.redirectSettingsWithError(c, "API ID 和 API Hash 不能为空")
			return
		}

		if displayName == "" {
			displayName = "Default API"
		}

		newCred, err := credSvc.Create(credential.CreateInput{
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

		// 确保新创建的记录是默认的
		if !newCred.IsDefault {
			credSvc.SetDefault(newCred.ID)
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
			Message:      "创建系统 API Key",
		})

		s.redirectSettingsWithSuccess(c, "API Key 配置已保存")
		return
	}

	// 已有系统 API Key，更新
	// 使用现有值作为默认，表单值覆盖
	updateName := systemKey.DisplayName
	if displayName != "" {
		updateName = displayName
	}
	updateAPIID := fmt.Sprintf("%d", systemKey.APIID)
	if apiIDStr != "" {
		updateAPIID = apiIDStr
	}

	_, err := credSvc.Update(systemKey.ID, credential.UpdateInput{
		DisplayName: updateName,
		APIID:       updateAPIID,
		APIHash:     apiHash, // 为空时 Update 内部保持不变
		Status:      string(systemKey.Status),
		RiskPolicy:  string(systemKey.RiskPolicy),
	})
	if err != nil {
		s.redirectSettingsWithError(c, "更新 API Key 失败: "+err.Error())
		return
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
		Message:      "更新系统 API Key 配置",
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
	testConfirmed := c.PostForm("test_result_confirmed") == "true"
	forceSave := c.PostForm("force_save") == "true"

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

		// 启用代理时要求检测确认
		if !testConfirmed {
			s.redirectSettingsWithError(c, "请先检测代理连通性")
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

	// 审计日志
	auditAction := "proxy.config_saved_after_successful_test"
	if forceSave {
		auditAction = "proxy.config_saved_after_failed_test"
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", adminID),
		Action:       auditAction,
		ResourceType: "settings",
		ResourceID:   "proxy",
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "代理设置已更新",
	})

	s.redirectSettingsWithSuccess(c, "代理配置已保存")
}

// handlePostProxyTest 处理 POST /settings/proxy/test - 检测代理连通性。
func (s *Server) handlePostProxyTest(c *gin.Context) {
	proxyType := c.PostForm("proxy_type")
	proxyHost := c.PostForm("proxy_host")
	proxyPort := c.PostForm("proxy_port")
	proxyUsername := c.PostForm("proxy_username")
	proxyPassword := c.PostForm("proxy_password")
	proxyTimeout := c.PostForm("proxy_timeout")

	// 如果是 none，直接返回成功
	if proxyType == "none" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"title":   "当前配置为不使用代理",
			"message": "Telegram 连接将尝试直连。",
		})
		return
	}

	// 校验
	if proxyType != "https" && proxyType != "socks5" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "invalid_config",
			"title":   "配置无效",
			"message": "不支持的代理类型。",
		})
		return
	}

	if proxyHost == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "invalid_config",
			"title":   "配置无效",
			"message": "代理主机不能为空。",
		})
		return
	}

	port, err := strconv.Atoi(proxyPort)
	if err != nil || port < 1 || port > 65535 {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "invalid_config",
			"title":   "配置无效",
			"message": "代理端口无效。",
		})
		return
	}

	timeout := 10 * time.Second
	if proxyTimeout != "" {
		if t, err := strconv.Atoi(proxyTimeout); err == nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
	}

	// 构建代理配置
	config := network.ProxyConfig{
		Type:     network.ProxyType(proxyType),
		Host:     proxyHost,
		Port:     port,
		Username: proxyUsername,
		Password: proxyPassword,
		Timeout:  timeout,
	}

	// 创建 dialer 并测试
	start := time.Now()
	dialer := network.NewDialer(config)

	// 测试连接 Telegram DC（使用公共测试目标）
	testTarget := "149.154.167.50:443" // Telegram DC
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", testTarget)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		// 分类错误
		code := "proxy_connect_failed"
		message := "无法连接到代理服务器，请检查主机地址和端口。"
		detail := ""

		errMsg := err.Error()
		if contains(errMsg, "auth") || contains(errMsg, "407") {
			code = "proxy_auth_failed"
			message = "代理认证失败，请检查用户名和密码。"
		} else if contains(errMsg, "timeout") || contains(errMsg, "deadline") {
			code = "timeout"
			message = "检测超时，请检查网络质量或代理可用性。"
		} else if contains(errMsg, "CONNECT") {
			code = "telegram_target_unreachable"
			message = "代理已连接，但无法访问 Telegram 测试目标。"
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":         false,
			"code":       code,
			"title":      "代理检测未通过",
			"message":    message,
			"detail":     detail,
			"elapsed_ms": elapsed,
		})
		return
	}
	conn.Close()

	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"title":      "代理检测通过",
		"message":    "当前代理配置可以建立连接。保存后，Atria 将使用该代理访问 Telegram MTProto API。",
		"elapsed_ms": elapsed,
	})
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

// handlePostUpdateCheckJSON 处理 POST /settings/update/check/json - 检查更新（JSON 响应）。
func (s *Server) handlePostUpdateCheckJSON(c *gin.Context) {
	info := s.handleGetSettingsUpdate(c)
	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"update_status": info["UpdateStatus"],
		"latest":        info["LatestVersion"],
		"message":       info["Message"],
		"is_docker":     info["IsDocker"],
	})
}

// handlePostUpdateDownloadJSON 处理 POST /settings/update/download/json - 下载更新（JSON 响应）。
func (s *Server) handlePostUpdateDownloadJSON(c *gin.Context) {
	s.handlePostUpdateDownload(c)
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "更新已下载完成。",
	})
}

// handlePostUpdateDryRunJSON 处理 POST /settings/update/dry-run/json - DryRun 验证（JSON 响应）。
func (s *Server) handlePostUpdateDryRunJSON(c *gin.Context) {
	s.handlePostUpdateDryRun(c)
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "DryRun 验证通过。",
	})
}

// handlePostUpdateApplyJSON 处理 POST /settings/update/apply/json - 应用更新（JSON 响应）。
func (s *Server) handlePostUpdateApplyJSON(c *gin.Context) {
	s.handlePostUpdateApply(c)
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "更新已应用，服务即将重启。",
	})
}
