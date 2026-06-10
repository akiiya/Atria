package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/chat"
	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/version"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// handleAPIMe 返回当前管理用户和 Telegram 账号状态。
func (s *Server) handleAPIMe(c *gin.Context) {
	adminUsername := auth.GetUsername(c)

	// 获取当前账号
	_, currentAccount := s.getAccountSwitcherData(c)

	// 获取所有活跃账号
	var accounts []model.TelegramAccount
	s.db.Where("status IN ?", []string{"active", "logged_out"}).
		Order("id ASC").Find(&accounts)

	type accountDTO struct {
		ID          uint   `json:"id"`
		DisplayName string `json:"display_name"`
		Username    string `json:"username"`
		AvatarText  string `json:"avatar_text"`
	}

	accountDTOs := make([]accountDTO, 0, len(accounts))
	for _, acc := range accounts {
		avatar := ""
		if acc.DisplayName != "" {
			avatar = string([]rune(acc.DisplayName)[0:1])
		}
		accountDTOs = append(accountDTOs, accountDTO{
			ID:          acc.ID,
			DisplayName: acc.DisplayName,
			Username:    acc.Username,
			AvatarText:  avatar,
		})
	}

	var currentDTO *accountDTO
	if currentAccount != nil {
		dto := accountDTO{
			ID:          currentAccount.ID,
			DisplayName: currentAccount.DisplayName,
			Username:    currentAccount.Username,
		}
		if currentAccount.DisplayName != "" {
			dto.AvatarText = string([]rune(currentAccount.DisplayName)[0:1])
		}
		currentDTO = &dto
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"admin":           gin.H{"username": adminUsername},
		"current_account": currentDTO,
		"accounts":        accountDTOs,
	})
}

// handleAPIDashboardStats 返回仪表盘统计。
func (s *Server) handleAPIDashboardStats(c *gin.Context) {
	var apiKeyCount int64
	s.db.Model(&model.APICredential{}).Where("status = ? AND deleted_at IS NULL", model.APICredentialStatusEnabled).Count(&apiKeyCount)

	var accountCount int64
	s.db.Model(&model.TelegramAccount{}).Where("status = ?", model.TelegramAccountStatusActive).Count(&accountCount)

	var sessionCount int64
	s.db.Model(&model.AccountSession{}).Where("status = ?", "active").Count(&sessionCount)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var auditTodayCount int64
	s.db.Model(&model.AuditLog{}).Where("created_at >= ?", todayStart).Count(&auditTodayCount)

	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"api_key_count": apiKeyCount,
		"account_count": accountCount,
		"session_count": sessionCount,
		"audit_today":   auditTodayCount,
		"version":       version.Short(),
		"db_driver":     s.cfg.DatabaseDriver,
		"data_dir":      s.cfg.DataDir,
		"listen_addr":   s.cfg.ListenAddr(),
	})
}

// handleAPIDialogs 返回聊天会话列表 JSON。
func (s *Server) handleAPIDialogs(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "no_current_account",
			"message": "请先接入 Telegram 账号",
			"dialogs": []interface{}{},
		})
		return
	}

	limit := 30
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	chatSvc := chat.NewChatService(s.db, s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	if dialer, _ := BuildProxyDialerFromDB(s.db, s.key); dialer != nil {
		chatSvc.SetProxyDialer(dialer)
	}

	result, err := chatSvc.ListDialogs(selectedID, limit)
	if err != nil {
		errMsg := s.classifyChatError(err)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "telegram_error",
			"message": errMsg,
			"dialogs": []interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"dialogs": result.Dialogs,
		"source":  result.Source,
		"stale":   result.Stale,
	})
}

// handleAPIMessages 返回消息历史 JSON。
func (s *Server) handleAPIMessages(c *gin.Context) {
	peerRef := c.Param("peer_ref")
	if peerRef == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "peer_invalid", "message": "缺少会话引用"})
		return
	}

	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "no_current_account", "message": "请先接入 Telegram 账号"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	chatSvc := chat.NewChatService(s.db, s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	if dialer, _ := BuildProxyDialerFromDB(s.db, s.key); dialer != nil {
		chatSvc.SetProxyDialer(dialer)
	}

	result, err := chatSvc.GetMessages(selectedID, peerRef, limit)
	if err != nil {
		errMsg := s.classifyChatError(err)
		errCode := "telegram_error"
		if chatErr, ok := err.(*chat.ChatError); ok {
			errCode = chatErr.Code
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":       false,
			"code":     errCode,
			"message":  errMsg,
			"messages": []interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"messages": result.Messages,
		"source":   result.Source,
		"stale":    result.Stale,
	})
}

// handleAPIAccounts 返回账号列表 JSON。
func (s *Server) handleAPIAccounts(c *gin.Context) {
	var accounts []model.TelegramAccount
	s.db.Preload("Session").Where("status IN ?", []string{"active", "logged_out", "banned", "restricted"}).
		Order("id ASC").Find(&accounts)

	type accountDTO struct {
		ID            uint   `json:"id"`
		DisplayName   string `json:"display_name"`
		Username      string `json:"username"`
		UserID        int64  `json:"user_id"`
		Status        string `json:"status"`
		SessionStatus string `json:"session_status"`
		LastSync      string `json:"last_sync"`
	}

	dtos := make([]accountDTO, 0, len(accounts))
	for _, acc := range accounts {
		dto := accountDTO{
			ID:          acc.ID,
			DisplayName: acc.DisplayName,
			Username:    acc.Username,
			UserID:      acc.UserID,
			Status:      string(acc.Status),
		}
		if acc.Session != nil {
			dto.SessionStatus = acc.Session.Status
		}
		if acc.LastSyncAt != nil {
			dto.LastSync = acc.LastSyncAt.Format("2006-01-02 15:04")
		}
		dtos = append(dtos, dto)
	}

	// 检查是否有 API Key
	var apiKeyCount int64
	s.db.Model(&model.APICredential{}).Where("status = ? AND deleted_at IS NULL", model.APICredentialStatusEnabled).Count(&apiKeyCount)

	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"accounts":    dtos,
		"has_api_key": apiKeyCount > 0,
	})
}

// handleAPIAccountDetail 返回账号详情 JSON。
func (s *Server) handleAPIAccountDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "invalid_id", "message": "无效的账号 ID"})
		return
	}

	var account model.TelegramAccount
	if err := s.db.Preload("Session").First(&account, uint(id)).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "not_found", "message": "账号不存在"})
		return
	}

	result := gin.H{
		"ok":             true,
		"id":             account.ID,
		"display_name":   account.DisplayName,
		"username":       account.Username,
		"user_id":        account.UserID,
		"status":         string(account.Status),
		"is_premium":     account.IsPremium,
		"is_restricted":  account.IsRestricted,
		"session_status": "",
		"last_sync":      "",
	}
	if account.Session != nil {
		result["session_status"] = account.Session.Status
	}
	if account.LastSyncAt != nil {
		result["last_sync"] = account.LastSyncAt.Format("2006-01-02 15:04:05")
	}

	c.JSON(http.StatusOK, result)
}

// handleAPIAudit 返回审计日志 JSON。
func (s *Server) handleAPIAudit(c *gin.Context) {
	var logs []model.AuditLog
	s.db.Order("id DESC").Limit(100).Find(&logs)

	type logDTO struct {
		ID           uint   `json:"id"`
		Action       string `json:"action"`
		ResourceType string `json:"resource_type"`
		ResourceID   uint   `json:"resource_id"`
		RiskLevel    string `json:"risk_level"`
		IP           string `json:"ip"`
		Message      string `json:"message"`
		CreatedAt    string `json:"created_at"`
	}

	dtos := make([]logDTO, 0, len(logs))
	for _, l := range logs {
		dtos = append(dtos, logDTO{
			ID:           l.ID,
			Action:       l.Action,
			ResourceType: l.ResourceType,
			ResourceID:   l.ResourceID,
			RiskLevel:    l.RiskLevel,
			IP:           l.IP,
			Message:      l.Message,
			CreatedAt:    l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"logs": dtos,
	})
}

// handleAPISettings 返回系统设置 JSON。
func (s *Server) handleAPISettings(c *gin.Context) {
	result := gin.H{
		"ok":          true,
		"version":     version.Short(),
		"db_driver":   s.cfg.DatabaseDriver,
		"data_dir":    s.cfg.DataDir,
		"listen_addr": s.cfg.ListenAddr(),
	}

	// API Key 信息
	credSvc := credential.NewService(s.db, s.key)
	systemKey, _ := credSvc.GetSystemAPIKey()
	if systemKey != nil {
		apiIDStr := fmt.Sprintf("%d", systemKey.APIID)
		apiIDMasked := apiIDStr
		if len(apiIDStr) > 4 {
			apiIDMasked = "****" + apiIDStr[len(apiIDStr)-4:]
		}
		result["api_key"] = gin.H{
			"display_name":  systemKey.DisplayName,
			"api_id_masked": apiIDMasked,
			"api_hash_hint": systemKey.APIHashHint,
		}
	}

	// 代理信息
	sm := settingMap(s.db)
	result["proxy"] = gin.H{
		"enabled":  sm["proxy_enabled"],
		"type":     sm["proxy_type"],
		"host":     sm["proxy_host"],
		"port":     sm["proxy_port"],
		"username": sm["proxy_username"],
		"timeout":  sm["proxy_timeout"],
		"remark":   sm["proxy_remark"],
	}

	c.JSON(http.StatusOK, result)
}

// settingMap 辅助函数：读取系统设置为 map。
func settingMap(db *gorm.DB) map[string]string {
	var settings []model.SystemSetting
	db.Find(&settings)
	m := make(map[string]string, len(settings))
	for _, s := range settings {
		m[s.Key] = s.Value
	}
	return m
}

// handleAPISaveProxy 处理 POST /api/settings/proxy - 保存代理配置（JSON）。
func (s *Server) handleAPISaveProxy(c *gin.Context) {
	var req struct {
		ProxyType     string `json:"proxy_type"`
		ProxyHost     string `json:"proxy_host"`
		ProxyPort     string `json:"proxy_port"`
		ProxyUsername string `json:"proxy_username"`
		ProxyPassword string `json:"proxy_password"`
		ProxyTimeout  string `json:"proxy_timeout"`
		ProxyRemark   string `json:"proxy_remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "请求格式错误"})
		return
	}

	// 校验
	if req.ProxyType != "none" && req.ProxyType != "https" && req.ProxyType != "socks5" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "无效的代理类型"})
		return
	}

	if req.ProxyType != "none" {
		if req.ProxyHost == "" {
			c.JSON(http.StatusOK, gin.H{"ok": false, "message": "代理主机不能为空"})
			return
		}
		port, err := strconv.Atoi(req.ProxyPort)
		if err != nil || port < 1 || port > 65535 {
			c.JSON(http.StatusOK, gin.H{"ok": false, "message": "无效的代理端口"})
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

	// 更新 proxy_enabled
	enabled := "false"
	if req.ProxyType != "none" {
		enabled = "true"
	}

	saveSetting("proxy_enabled", enabled, false)
	saveSetting("proxy_type", req.ProxyType, false)
	saveSetting("proxy_host", req.ProxyHost, false)
	saveSetting("proxy_port", req.ProxyPort, false)
	saveSetting("proxy_username", req.ProxyUsername, false)
	saveSetting("proxy_timeout", req.ProxyTimeout, false)
	saveSetting("proxy_remark", req.ProxyRemark, false)

	// 只有提供了新密码才更新
	if req.ProxyPassword != "" {
		encryptedPassword, err := crypto.EncryptString(s.key, req.ProxyPassword, []byte("atria:proxy:v1"))
		if err != nil {
			slog.Error("加密代理密码失败", "error", err)
		} else {
			saveSetting("proxy_password", encryptedPassword, true)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "代理配置已保存"})
}

// handleAPISaveAPIKey 处理 POST /api/settings/api-key - 保存 API Key（JSON）。
func (s *Server) handleAPISaveAPIKey(c *gin.Context) {
	var req struct {
		DisplayName string `json:"display_name"`
		APIID       string `json:"api_id"`
		APIHash     string `json:"api_hash"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "请求格式错误"})
		return
	}

	credSvc := credential.NewService(s.db, s.key)
	systemKey, _ := credSvc.GetSystemAPIKey()

	if systemKey == nil {
		// 创建新的
		if req.APIID == "" || req.APIHash == "" {
			c.JSON(http.StatusOK, gin.H{"ok": false, "message": "API ID 和 API Hash 不能为空"})
			return
		}
		if req.DisplayName == "" {
			req.DisplayName = "Default API"
		}
		newCred, err := credSvc.Create(credential.CreateInput{
			DisplayName: req.DisplayName,
			APIID:       req.APIID,
			APIHash:     req.APIHash,
			Status:      "enabled",
			RiskPolicy:  "disabled",
		})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "message": "创建 API Key 失败: " + err.Error()})
			return
		}
		if !newCred.IsDefault {
			credSvc.SetDefault(newCred.ID)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "API Key 已保存"})
		return
	}

	// 更新现有
	updateName := systemKey.DisplayName
	if req.DisplayName != "" {
		updateName = req.DisplayName
	}
	updateAPIID := fmt.Sprintf("%d", systemKey.APIID)
	if req.APIID != "" {
		updateAPIID = req.APIID
	}

	_, err := credSvc.Update(systemKey.ID, credential.UpdateInput{
		DisplayName: updateName,
		APIID:       updateAPIID,
		APIHash:     req.APIHash, // 为空时保持不变
		Status:      string(systemKey.Status),
		RiskPolicy:  string(systemKey.RiskPolicy),
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新 API Key 失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "API Key 已保存"})
}
