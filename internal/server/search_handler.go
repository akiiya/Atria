package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/user/atria/internal/chat"
)

// handleAPISearchMessages 搜索本地消息缓存。
func (s *Server) handleAPISearchMessages(c *gin.Context) {
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "no_current_account",
			"message": "请先接入 Telegram 账号",
		})
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"results": []chat.SearchResult{},
			"total":   0,
		})
		return
	}

	peerRef := c.Query("peer_ref")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	chatSvc := s.newChatService()
	results, total, err := chatSvc.SearchMessages(c.Request.Context(), selectedID, q, peerRef, limit, offset)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "搜索失败"})
		return
	}

	if results == nil {
		results = []chat.SearchResult{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"results": results,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
