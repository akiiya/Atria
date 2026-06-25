package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/media"
	"github.com/user/atria/internal/security"
)

func (s *Server) handleAPIMediaStatus(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "no_current_account", "message": "请先接入 Telegram 账号"})
		return
	}

	messageID, _ := strconv.Atoi(c.Param("message_id"))
	peerRef := c.Query("peer_ref")
	if messageID == 0 || peerRef == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "缺少 message_id 或 peer_ref"})
		return
	}

	mediaSvc := s.newMediaService()
	status, err := mediaSvc.GetMediaStatus(c.Request.Context(), selectedID, peerRef, messageID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "查询媒体状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"status":    status.Status,
		"file_name": status.FileName,
		"mime_type": status.MIMEType,
		"file_size": status.FileSize,
		"available": status.Available,
	})
}

func (s *Server) handleAPIMediaDownload(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "no_current_account", "message": "请先接入 Telegram 账号"})
		return
	}

	messageID, _ := strconv.Atoi(c.Param("message_id"))
	peerRef := c.Query("peer_ref")
	if messageID == 0 || peerRef == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "缺少 message_id 或 peer_ref"})
		return
	}

	// 获取 peer 信息
	chatSvc := s.newChatService()
	peerCache, err := chatSvc.GetPeerCache(selectedID, peerRef)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "会话信息不存在"})
		return
	}

	// 获取账号凭据
	account, cred, err := chatSvc.GetAccountAndCredential(selectedID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "账号信息不存在"})
		return
	}

	apiHash, _ := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	accessHash, _ := chatSvc.DecryptAccessHash(peerCache.AccessHashEncrypted)

	mediaSvc := s.newMediaService()
	result, err := mediaSvc.DownloadMedia(c.Request.Context(), selectedID, peerRef, messageID,
		peerCache.PeerID, peerCache.PeerType, accessHash, int(cred.APIID), apiHash, account.Session.SessionFilePath)

	if err != nil {
		errMsg := security.SanitizeErrorMessage(err.Error())
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "下载失败: " + errMsg})
		return
	}

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "media.download",
		ResourceType: "media",
		AccountID:    selectedID,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		Metadata: map[string]any{
			"account_id": selectedID,
			"peer_ref":   peerRef,
			"message_id": messageID,
			"file_name":  result.FileName,
			"file_size":  result.FileSize,
		},
		Message: fmt.Sprintf("下载媒体: %s (%d bytes)", result.FileName, result.FileSize),
	})

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"status":    result.Status,
		"file_name": result.FileName,
		"mime_type": result.MIMEType,
		"file_size": result.FileSize,
	})
}

func (s *Server) handleAPIMediaContent(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "no_current_account", "message": "请先接入 Telegram 账号"})
		return
	}

	messageID, _ := strconv.Atoi(c.Param("message_id"))
	peerRef := c.Query("peer_ref")
	if messageID == 0 || peerRef == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "缺少 message_id 或 peer_ref"})
		return
	}

	mediaSvc := s.newMediaService()
	filePath, cache, err := mediaSvc.GetMediaContent(c.Request.Context(), selectedID, peerRef, messageID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "媒体不可用"})
		return
	}

	c.Header("Content-Type", cache.MIMEType)
	if cache.FileName != "" {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", cache.FileName))
	}
	c.File(filePath)
}

// Ensure media.Service is used (import check).
var _ = (*media.Service)(nil)
