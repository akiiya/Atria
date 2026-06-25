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
	"github.com/user/atria/internal/chat"
	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/security"
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

	// Runtime 状态统计
	var runtimeLive, runtimeOffline, runtimeStopped int
	if s.runtimeManager != nil {
		var accounts []model.TelegramAccount
		s.db.Where("status = ?", model.TelegramAccountStatusActive).Find(&accounts)
		for _, acc := range accounts {
			status := s.runtimeManager.Status(acc.ID)
			switch status.State {
			case "live", "syncing":
				runtimeLive++
			case "connecting", "degraded":
				runtimeOffline++
			case "stopped", "offline":
				runtimeStopped++
			}
		}
	}

	// 近 24 小时错误数（risk_level high/critical）
	var recentErrors int64
	s.db.Model(&model.AuditLog{}).
		Where("risk_level IN ? AND created_at > ?", []string{"high", "critical"}, now.Add(-24*time.Hour)).
		Count(&recentErrors)

	// 近 24 小时审计事件总数
	var recentAuditCount int64
	s.db.Model(&model.AuditLog{}).
		Where("created_at > ?", now.Add(-24*time.Hour)).
		Count(&recentAuditCount)

	// 最近 5 条审计日志
	var recentLogs []model.AuditLog
	s.db.Order("id DESC").Limit(5).Find(&recentLogs)

	type recentLogDTO struct {
		ID        uint   `json:"id"`
		Action    string `json:"action"`
		Message   string `json:"message"`
		RiskLevel string `json:"risk_level"`
		CreatedAt string `json:"created_at"`
	}
	recentLogDTOs := make([]recentLogDTO, 0, len(recentLogs))
	for _, l := range recentLogs {
		recentLogDTOs = append(recentLogDTOs, recentLogDTO{
			ID:        l.ID,
			Action:    l.Action,
			Message:   l.Message,
			RiskLevel: l.RiskLevel,
			CreatedAt: l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"api_key_count":   apiKeyCount,
		"account_count":   accountCount,
		"session_count":   sessionCount,
		"audit_today":     auditTodayCount,
		"runtime_live":    runtimeLive,
		"runtime_offline": runtimeOffline,
		"runtime_stopped": runtimeStopped,
		"recent_errors":   recentErrors,
		"recent_audit":    recentAuditCount,
		"recent_logs":     recentLogDTOs,
		"version":         version.Short(),
		"db_driver":       s.cfg.DatabaseDriver,
		"data_dir":        s.cfg.DataDir,
		"listen_addr":     s.cfg.ListenAddr(),
	})
}

// handleAPIContacts 返回联系人列表 JSON。
func (s *Server) handleAPIContacts(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":       false,
			"code":     "no_current_account",
			"message":  "请先接入 Telegram 账号",
			"contacts": []interface{}{},
		})
		return
	}

	forceRefresh := c.Query("force_refresh") == "true"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	chatSvc := s.newChatService()
	result, err := chatSvc.GetContacts(ctx, selectedID, forceRefresh)
	if err != nil {
		errMsg := s.classifyChatError(err)
		errCode := "telegram_error"
		if chatErr, ok := err.(*chat.ChatError); ok {
			errCode = chatErr.Code
		}
		if c.Request.Context().Err() != nil {
			errCode = "request_timeout"
			errMsg = "请求超时，请稍后重试"
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":       false,
			"code":     errCode,
			"message":  errMsg,
			"contacts": []interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"contacts": result.Contacts,
		"source":   result.Source,
		"stale":    result.Stale,
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

	// 15 秒超时：即使 Telegram/runtime 卡住，客户端也不会无限等待
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	chatSvc := s.newChatService()

	// force_refresh: 用户主动刷新时跳过缓存
	forceRefresh := c.Query("force_refresh") == "true"

	result, err := chatSvc.ListDialogs(ctx, selectedID, limit, forceRefresh)
	if err != nil {
		errMsg := s.classifyChatError(err)
		errCode := "telegram_error"
		if c.Request.Context().Err() != nil {
			errCode = "request_timeout"
			errMsg = "请求超时，请稍后重试"
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    errCode,
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
// 支持 before_id 参数用于分页加载更早消息。
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
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// 解析 before_id 参数
	beforeID := 0
	if b := c.Query("before_id"); b != "" {
		if parsed, err := strconv.Atoi(b); err == nil && parsed > 0 {
			beforeID = parsed
		}
	}

	chatSvc := s.newChatService()

	// force_refresh: 用户主动刷新时跳过缓存
	forceRefresh := c.Query("force_refresh") == "true"

	// 15 秒超时
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var result *chat.MessagesResult
	var err error

	if beforeID > 0 {
		// 加载更早消息
		result, err = chatSvc.LoadOlderMessages(ctx, selectedID, peerRef, beforeID, limit, forceRefresh)
	} else {
		// 加载最近消息
		result, err = chatSvc.GetMessages(ctx, selectedID, peerRef, limit, forceRefresh)
	}

	if err != nil {
		errMsg := s.classifyChatError(err)
		errCode := "telegram_error"
		if chatErr, ok := err.(*chat.ChatError); ok {
			errCode = chatErr.Code
		}
		if c.Request.Context().Err() != nil {
			errCode = "request_timeout"
			errMsg = "请求超时，请稍后重试"
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
		"ok":                true,
		"messages":          result.Messages,
		"source":            result.Source,
		"stale":             result.Stale,
		"has_older":         result.HasOlder,
		"oldest_message_id": result.OldestMessageID,
		"newest_message_id": result.NewestMessageID,
	})
}

// handleAPIAccounts 返回账号列表 JSON，包含运行时状态。
func (s *Server) handleAPIAccounts(c *gin.Context) {
	var accounts []model.TelegramAccount
	s.db.Preload("Session").Where("status IN ?", []string{"active", "logged_out", "banned", "restricted", "disabled"}).
		Order("id ASC").Find(&accounts)

	type accountDTO struct {
		ID               uint   `json:"id"`
		DisplayName      string `json:"display_name"`
		Username         string `json:"username"`
		UserID           int64  `json:"user_id"`
		Status           string `json:"status"`
		SessionStatus    string `json:"session_status"`
		RuntimeState     string `json:"runtime_state"`
		LastError        string `json:"last_error,omitempty"`
		IsCurrentAccount bool   `json:"is_current_account"`
		LastSync         string `json:"last_sync"`
		UpdatedAt        string `json:"updated_at"`
	}

	selectedID := s.resolveCurrentAccountID(c)

	dtos := make([]accountDTO, 0, len(accounts))
	for _, acc := range accounts {
		dto := accountDTO{
			ID:               acc.ID,
			DisplayName:      acc.DisplayName,
			Username:         acc.Username,
			UserID:           acc.UserID,
			Status:           string(acc.Status),
			IsCurrentAccount: acc.ID == selectedID,
		}
		if acc.Session != nil {
			dto.SessionStatus = acc.Session.Status
		}
		if acc.LastSyncAt != nil {
			dto.LastSync = acc.LastSyncAt.Format("2006-01-02 15:04")
		}
		dto.UpdatedAt = acc.UpdatedAt.Format("2006-01-02 15:04")

		// 运行时状态
		if s.runtimeManager != nil {
			rtStatus := s.runtimeManager.Status(acc.ID)
			dto.RuntimeState = string(rtStatus.State)
			if rtStatus.LastError != "" {
				dto.LastError = security.SanitizeErrorMessage(rtStatus.LastError)
			}
		} else {
			dto.RuntimeState = "unknown"
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

// handleAPIAudit 返回审计日志 JSON，支持过滤和分页。
func (s *Server) handleAPIAudit(c *gin.Context) {
	query := s.db.Model(&model.AuditLog{})

	// 过滤条件（支持 event_type 作为 action 的别名）
	if eventType := c.Query("event_type"); eventType != "" {
		query = query.Where("action = ?", eventType)
	} else if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if accountID := c.Query("account_id"); accountID != "" {
		if id, err := strconv.Atoi(accountID); err == nil && id > 0 {
			query = query.Where("account_id = ?", id)
		}
	}
	if riskLevel := c.Query("risk_level"); riskLevel != "" {
		query = query.Where("risk_level = ?", riskLevel)
	}
	if since := c.Query("since"); since != "" {
		query = query.Where("created_at >= ?", since)
	}
	if until := c.Query("until"); until != "" {
		query = query.Where("created_at <= ?", until)
	}

	// 分页
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// 总数
	var total int64
	query.Count(&total)

	// 查询
	var logs []model.AuditLog
	query.Order("id DESC").Limit(limit).Offset(offset).Find(&logs)

	type logDTO struct {
		ID           uint   `json:"id"`
		AccountID    uint   `json:"account_id"`
		Action       string `json:"action"`
		ResourceType string `json:"resource_type"`
		ResourceID   uint   `json:"resource_id"`
		RiskLevel    string `json:"risk_level"`
		IP           string `json:"ip"`
		Message      string `json:"message"`
		MetadataJSON string `json:"metadata_json"`
		CreatedAt    string `json:"created_at"`
	}

	dtos := make([]logDTO, 0, len(logs))
	for _, l := range logs {
		dtos = append(dtos, logDTO{
			ID:           l.ID,
			AccountID:    l.AccountID,
			Action:       l.Action,
			ResourceType: l.ResourceType,
			ResourceID:   l.ResourceID,
			RiskLevel:    l.RiskLevel,
			IP:           l.IP,
			Message:      l.Message,
			MetadataJSON: l.MetadataJSON,
			CreatedAt:    l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":     true,
		"logs":   dtos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// handleAPIAuditEventTypes 返回已使用的审计事件类型列表（仅 value + count，标签由前端 i18n 提供）。
func (s *Server) handleAPIAuditEventTypes(c *gin.Context) {
	type eventTypeRow struct {
		Action string
		Count  int64
	}
	var rows []eventTypeRow
	s.db.Model(&model.AuditLog{}).
		Select("action, count(*) as count").
		Group("action").
		Order("count DESC").
		Find(&rows)

	type eventTypeDTO struct {
		Value string `json:"value"`
		Count int64  `json:"count"`
	}
	dtos := make([]eventTypeDTO, 0, len(rows))
	for _, r := range rows {
		dtos = append(dtos, eventTypeDTO{Value: r.Action, Count: r.Count})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"event_types": dtos,
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
	proxyType := sm["proxy_type"]

	// 检测 legacy api_proxy 配置
	proxyValid := proxyType != "api_proxy"
	proxyLegacyMessage := ""
	if !proxyValid {
		proxyLegacyMessage = "API Proxy 已移除，不适用于 MTProto 连接，请重新选择 SOCKS5 或 HTTPS CONNECT 代理"
	}

	result["proxy"] = gin.H{
		"enabled":        sm["proxy_enabled"],
		"type":           proxyType,
		"host":           sm["proxy_host"],
		"port":           sm["proxy_port"],
		"username":       sm["proxy_username"],
		"timeout":        sm["proxy_timeout"],
		"remark":         sm["proxy_remark"],
		"proxy_valid":    proxyValid,
		"legacy_message": proxyLegacyMessage,
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

	// 校验：api_proxy 已移除
	if req.ProxyType == "api_proxy" {
		c.JSON(http.StatusOK, gin.H{
			"ok":          false,
			"code":        "proxy_type_removed",
			"message":     "API Proxy 已移除，当前 Telegram 登录/聊天基于 MTProto，请使用 SOCKS5 或 HTTPS CONNECT 代理",
			"proxy_valid": false,
		})
		return
	}

	if req.ProxyType != "none" && req.ProxyType != "https" && req.ProxyType != "socks5" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "无效的代理类型"})
		return
	}

	// socks5/https 类型校验主机和端口
	if req.ProxyType == "socks5" || req.ProxyType == "https" {
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

	// 保存有效代理配置，同时清除 legacy api_proxy_url
	saveSetting("proxy_host", req.ProxyHost, false)
	saveSetting("proxy_port", req.ProxyPort, false)
	saveSetting("proxy_username", req.ProxyUsername, false)
	saveSetting("proxy_timeout", req.ProxyTimeout, false)
	saveSetting("proxy_remark", req.ProxyRemark, false)

	// 清除 legacy api_proxy_url（如有）
	saveSetting("api_proxy_url", "", false)

	// 只有提供了新密码才更新
	if req.ProxyPassword != "" {
		encryptedPassword, err := crypto.EncryptString(s.key, req.ProxyPassword, []byte("atria:proxy:v1"))
		if err != nil {
			slog.Error("加密代理密码失败", "error", err)
		} else {
			saveSetting("proxy_password", encryptedPassword, true)
		}
	}

	// 通知 RuntimeManager 代理配置已变更
	// 1. 重建 dialer
	// 2. 停止所有运行时（它们会用旧 dialer）
	available, proxyErr := s.runtimeManager.OnProxySettingsChanged(s.db, s.key)

	// 构建响应
	response := gin.H{"ok": true, "message": "代理配置已保存"}
	if proxyErr != nil {
		response["warning"] = proxyErr.Error()
		response["proxy_available"] = false
	} else {
		response["proxy_available"] = available
	}

	c.JSON(http.StatusOK, response)
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

// handleAPIRuntimeStatus 返回当前 selected account 的 runtime 状态。
func (s *Server) handleAPIRuntimeStatus(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "no_current_account",
			"message": "请先接入 Telegram 账号",
		})
		return
	}

	status := s.runtimeManager.Status(selectedID)

	// executor_ready: 只在 live/syncing 时为 true（connecting 时 Run() 未启动）
	executorReady := status.State == "live" || status.State == "syncing"

	// last_error 脱敏：移除可能包含的敏感路径和凭据
	lastErr := status.LastError
	if lastErr != "" {
		lastErr = security.SanitizeErrorMessage(lastErr)
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":             true,
		"account_id":     status.AccountID,
		"state":          string(status.State),
		"executor_ready": executorReady,
		"last_sync_at":   formatTimePtr(status.LastSyncAt),
		"last_event_at":  formatTimePtr(status.LastEventAt),
		"last_error":     lastErr,
		"active":         status.State != "stopped",
	})
}

// handleAPIRuntimeStart 启动 runtime。支持通过 JSON body 传入 account_id。
func (s *Server) handleAPIRuntimeStart(c *gin.Context) {
	// 尝试从请求体读取 account_id
	var req struct {
		AccountID uint `json:"account_id"`
	}
	c.ShouldBindJSON(&req) // 忽略错误，body 可能为空

	accountID := req.AccountID
	if accountID == 0 {
		accountID = s.resolveCurrentAccountID(c)
	}
	if accountID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "no_current_account",
			"message": "请先接入 Telegram 账号",
		})
		return
	}

	err := s.runtimeManager.StartAccount(accountID)
	if err != nil {
		slog.Error("启动 runtime 失败", "account_id", accountID, "error", err)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "runtime_start_failed",
			"message": "启动运行时失败: " + err.Error(),
		})
		return
	}

	status := s.runtimeManager.Status(accountID)
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "runtime.start",
		ResourceType: "account",
		AccountID:    accountID,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		Message:      fmt.Sprintf("启动 runtime (account_id=%d)", accountID),
	})
	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"account_id": accountID,
		"state":      string(status.State),
	})
}

// handleAPIRuntimeStop 停止 runtime。支持通过 JSON body 传入 account_id。
func (s *Server) handleAPIRuntimeStop(c *gin.Context) {
	// 尝试从请求体读取 account_id
	var req struct {
		AccountID uint `json:"account_id"`
	}
	c.ShouldBindJSON(&req) // 忽略错误，body 可能为空

	accountID := req.AccountID
	if accountID == 0 {
		accountID = s.resolveCurrentAccountID(c)
	}
	if accountID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "no_current_account",
			"message": "请先接入 Telegram 账号",
		})
		return
	}

	err := s.runtimeManager.StopAccount(accountID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "runtime_stop_failed",
			"message": "停止运行时失败: " + err.Error(),
		})
		return
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "runtime.stop",
		ResourceType: "account",
		AccountID:    accountID,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		Message:      fmt.Sprintf("停止 runtime (account_id=%d)", accountID),
	})
	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"account_id": accountID,
		"state":      "stopped",
	})
}

// handleAPIAccountEnable 启用指定账号。
func (s *Server) handleAPIAccountEnable(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "无效的账号 ID"})
		return
	}

	var account model.TelegramAccount
	if err := s.db.First(&account, uint(id)).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "账号不存在"})
		return
	}

	if account.Status == model.TelegramAccountStatusActive {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "账号已是启用状态"})
		return
	}

	s.db.Model(&account).Update("status", model.TelegramAccountStatusActive)
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "account.enable",
		ResourceType: "account",
		ResourceID:   idStr,
		AccountID:    uint(id),
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		Message:      fmt.Sprintf("启用账号 %s (id=%d)", account.DisplayName, id),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "账号已启用"})
}

// handleAPIAccountDisable 禁用指定账号。禁用前会停止 runtime。
func (s *Server) handleAPIAccountDisable(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "无效的账号 ID"})
		return
	}

	var account model.TelegramAccount
	if err := s.db.First(&account, uint(id)).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "账号不存在"})
		return
	}

	if account.Status != model.TelegramAccountStatusActive {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "账号已是禁用状态"})
		return
	}

	// 先停止 runtime
	if s.runtimeManager != nil {
		s.runtimeManager.StopAccount(uint(id))
	}

	s.db.Model(&account).Update("status", "disabled")
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "account.disable",
		ResourceType: "account",
		ResourceID:   idStr,
		AccountID:    uint(id),
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		Message:      fmt.Sprintf("禁用账号 %s (id=%d)", account.DisplayName, id),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "账号已禁用"})
}

// formatTimePtr 格式化时间指针为 ISO 字符串。
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
