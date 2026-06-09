package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/chat"
	"github.com/user/atria/internal/model"

	"github.com/gin-gonic/gin"
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

	dialogs, err := chatSvc.ListDialogs(selectedID, limit)
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
		"dialogs": dialogs,
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

	messages, err := chatSvc.GetMessages(selectedID, peerRef, limit)
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

	// 反转消息顺序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"messages": messages,
	})
}
