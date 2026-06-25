package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/model"
)

// handleAPIMaintenanceStatus 返回系统维护状态。
func (s *Server) handleAPIMaintenanceStatus(c *gin.Context) {
	// 表统计
	var accountCount, peerCacheCount, messageCacheCount, auditLogCount, apiKeyCount int64
	s.db.Model(&model.TelegramAccount{}).Count(&accountCount)
	s.db.Model(&model.ChatPeerCache{}).Count(&peerCacheCount)
	s.db.Model(&model.ChatMessageCache{}).Count(&messageCacheCount)
	s.db.Model(&model.AuditLog{}).Count(&auditLogCount)
	s.db.Model(&model.APICredential{}).Count(&apiKeyCount)

	// Orphan peer cache（没有对应 active account 的 peer cache）
	var orphanPeers int64
	s.db.Model(&model.ChatPeerCache{}).
		Where("account_id NOT IN (SELECT id FROM telegram_accounts WHERE status = 'active')").
		Count(&orphanPeers)

	// Orphan message cache（没有对应 active account 的 message cache）
	var orphanMessages int64
	s.db.Model(&model.ChatMessageCache{}).
		Where("account_id NOT IN (SELECT id FROM telegram_accounts WHERE status = 'active')").
		Count(&orphanMessages)

	// 最近维护操作
	var recentMaintenance []model.AuditLog
	s.db.Where("action LIKE ?", "maintenance.%").
		Order("id DESC").Limit(5).Find(&recentMaintenance)

	type recentDTO struct {
		ID        uint   `json:"id"`
		Action    string `json:"action"`
		Message   string `json:"message"`
		CreatedAt string `json:"created_at"`
	}
	recentDTOs := make([]recentDTO, 0, len(recentMaintenance))
	for _, l := range recentMaintenance {
		recentDTOs = append(recentDTOs, recentDTO{
			ID:        l.ID,
			Action:    l.Action,
			Message:   l.Message,
			CreatedAt: l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	// 迁移版本（从 data_migrations 表取最新）
	var currentVersion int
	s.db.Raw("SELECT COALESCE(MAX(version), 0) FROM data_migrations").Scan(&currentVersion)

	c.JSON(http.StatusOK, gin.H{
		"ok":                  true,
		"account_count":       accountCount,
		"api_key_count":       apiKeyCount,
		"peer_cache_count":    peerCacheCount,
		"message_cache_count": messageCacheCount,
		"audit_log_count":     auditLogCount,
		"orphan_peers":        orphanPeers,
		"orphan_messages":     orphanMessages,
		"migration_version":   currentVersion,
		"recent_maintenance":  recentDTOs,
	})
}

// handleAPICleanupChatCache 清理聊天缓存。
func (s *Server) handleAPICleanupChatCache(c *gin.Context) {
	var req struct {
		AccountID uint   `json:"account_id"`
		PeerRef   string `json:"peer_ref"`
		DryRun    *bool  `json:"dry_run"` // 必须显式传 false 才执行
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "请求格式错误"})
		return
	}

	if req.DryRun == nil || *req.DryRun {
		// dry-run：只统计将删除的数量
		peerCount, msgCount := s.countChatCacheForCleanup(req.AccountID, req.PeerRef)
		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"dry_run":    true,
			"peer_count": peerCount,
			"msg_count":  msgCount,
			"message":    fmt.Sprintf("将删除 %d 条 peer 缓存和 %d 条消息缓存", peerCount, msgCount),
		})
		return
	}

	// 真正执行
	if req.AccountID == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "account_id 不能为空"})
		return
	}

	peerCount, msgCount := s.cleanupChatCache(req.AccountID, req.PeerRef)

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "maintenance.cleanup_chat_cache",
		ResourceType: "cache",
		AccountID:    req.AccountID,
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		Message:      fmt.Sprintf("清理聊天缓存: account=%d peer=%s, 删除 %d peer + %d message", req.AccountID, req.PeerRef, peerCount, msgCount),
	})

	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"dry_run":    false,
		"peer_count": peerCount,
		"msg_count":  msgCount,
		"message":    fmt.Sprintf("已删除 %d 条 peer 缓存和 %d 条消息缓存", peerCount, msgCount),
	})
}

func (s *Server) countChatCacheForCleanup(accountID uint, peerRef string) (int64, int64) {
	var peerCount, msgCount int64
	if peerRef != "" {
		s.db.Model(&model.ChatPeerCache{}).Where("account_id = ? AND peer_ref = ?", accountID, peerRef).Count(&peerCount)
		s.db.Model(&model.ChatMessageCache{}).Where("account_id = ? AND peer_ref = ?", accountID, peerRef).Count(&msgCount)
	} else {
		s.db.Model(&model.ChatPeerCache{}).Where("account_id = ?", accountID).Count(&peerCount)
		s.db.Model(&model.ChatMessageCache{}).Where("account_id = ?", accountID).Count(&msgCount)
	}
	return peerCount, msgCount
}

func (s *Server) cleanupChatCache(accountID uint, peerRef string) (int64, int64) {
	var peerCount, msgCount int64
	if peerRef != "" {
		result := s.db.Where("account_id = ? AND peer_ref = ?", accountID, peerRef).Delete(&model.ChatPeerCache{})
		peerCount = result.RowsAffected
		result = s.db.Where("account_id = ? AND peer_ref = ?", accountID, peerRef).Delete(&model.ChatMessageCache{})
		msgCount = result.RowsAffected
	} else {
		result := s.db.Where("account_id = ?", accountID).Delete(&model.ChatPeerCache{})
		peerCount = result.RowsAffected
		result = s.db.Where("account_id = ?", accountID).Delete(&model.ChatMessageCache{})
		msgCount = result.RowsAffected
	}
	return peerCount, msgCount
}

// handleAPICleanupOrphans 清理孤立缓存。
func (s *Server) handleAPICleanupOrphans(c *gin.Context) {
	var req struct {
		DryRun *bool `json:"dry_run"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "请求格式错误"})
		return
	}

	if req.DryRun == nil || *req.DryRun {
		orphanPeers, orphanMsgs := s.countOrphans()
		c.JSON(http.StatusOK, gin.H{
			"ok":              true,
			"dry_run":         true,
			"orphan_peers":    orphanPeers,
			"orphan_messages": orphanMsgs,
			"message":         fmt.Sprintf("将删除 %d 条孤立 peer 缓存和 %d 条孤立消息缓存", orphanPeers, orphanMsgs),
		})
		return
	}

	orphanPeers, orphanMsgs := s.cleanupOrphans()

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "maintenance.cleanup_orphans",
		ResourceType: "cache",
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		Message:      fmt.Sprintf("清理孤立缓存: 删除 %d peer + %d message", orphanPeers, orphanMsgs),
	})

	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"dry_run":         false,
		"orphan_peers":    orphanPeers,
		"orphan_messages": orphanMsgs,
		"message":         fmt.Sprintf("已删除 %d 条孤立 peer 缓存和 %d 条孤立消息缓存", orphanPeers, orphanMsgs),
	})
}

func (s *Server) countOrphans() (int64, int64) {
	var orphanPeers, orphanMsgs int64
	s.db.Model(&model.ChatPeerCache{}).
		Where("account_id NOT IN (SELECT id FROM telegram_accounts WHERE status = 'active')").
		Count(&orphanPeers)
	s.db.Model(&model.ChatMessageCache{}).
		Where("account_id NOT IN (SELECT id FROM telegram_accounts WHERE status = 'active')").
		Count(&orphanMsgs)
	return orphanPeers, orphanMsgs
}

func (s *Server) cleanupOrphans() (int64, int64) {
	r1 := s.db.Where("account_id NOT IN (SELECT id FROM telegram_accounts WHERE status = 'active')").Delete(&model.ChatPeerCache{})
	r2 := s.db.Where("account_id NOT IN (SELECT id FROM telegram_accounts WHERE status = 'active')").Delete(&model.ChatMessageCache{})
	return r1.RowsAffected, r2.RowsAffected
}
