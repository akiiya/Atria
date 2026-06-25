package server

import (
	"net/http"

	"github.com/user/atria/internal/model"

	"github.com/gin-gonic/gin"
)

// handleAPIPeerInfo 返回 peer 详细信息。
func (s *Server) handleAPIPeerInfo(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "no_current_account", "message": "请先接入 Telegram 账号"})
		return
	}

	peerRef := c.Param("peer_ref")
	if peerRef == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "缺少 peer_ref"})
		return
	}

	// 从 peer cache 读取
	var cache model.ChatPeerCache
	err := s.db.Where("account_id = ? AND peer_ref = ?", selectedID, peerRef).First(&cache).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "会话信息不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"peer_ref":     cache.PeerRef,
		"peer_type":    cache.PeerType,
		"title":        cache.Title,
		"username":     cache.Username,
		"member_count": cache.MemberCount,
		"flags":        cache.Flags,
		"description":  cache.Description,
		"source":       "cache",
	})
}
