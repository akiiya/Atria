package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/chat"
	"github.com/user/atria/internal/mtproto"

	"github.com/gin-gonic/gin"
)

// handleGetChats 处理 GET /chats - 会话列表页面。
func (s *Server) handleGetChats(c *gin.Context) {
	data := s.newAccountViewData(c, "chats")

	// 使用统一的当前账号解析（带 fallback）
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		data["NoCurrentAccount"] = true
		c.HTML(http.StatusOK, "chats.html", data)
		return
	}

	data["CurrentAccountID"] = selectedID
	data["HasCurrentAccount"] = true

	// 获取会话列表
	chatSvc := s.newChatService()

	dialogs, err := chatSvc.ListDialogs(c.Request.Context(), selectedID, 20, false)
	if err != nil {
		slog.Error("获取会话列表失败", "error", err)
		errMsg := s.classifyChatError(err)
		data["Error"] = errMsg
		data["Dialogs"] = []chat.Dialog{}
		c.HTML(http.StatusOK, "chats.html", data)
		return
	}

	data["Dialogs"] = dialogs
	c.HTML(http.StatusOK, "chats.html", data)
}

// handleGetChatDetail 处理 GET /chats/:peer_ref - 消息历史页面。
func (s *Server) handleGetChatDetail(c *gin.Context) {
	peerRef := c.Param("peer_ref")
	if peerRef == "" {
		RenderError(c, http.StatusBadRequest, "请求无效", "缺少会话引用")
		return
	}

	data := s.newAccountViewData(c, "chats")

	// 使用统一的当前账号解析（带 fallback）
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		data["NoCurrentAccount"] = true
		c.HTML(http.StatusOK, "chat_detail.html", data)
		return
	}

	data["CurrentAccountID"] = selectedID
	data["HasCurrentAccount"] = true
	data["PeerRef"] = peerRef

	// 获取消息历史
	chatSvc := s.newChatService()

	result, err := chatSvc.GetMessages(c.Request.Context(), selectedID, peerRef, 50, false)
	if err != nil {
		slog.Error("获取消息历史失败", "error", err, "peer_ref_length", len(peerRef))
		errMsg := s.classifyChatError(err)
		data["Error"] = errMsg
		data["Messages"] = []chat.Message{}
		c.HTML(http.StatusOK, "chat_detail.html", data)
		return
	}

	data["Messages"] = result.Messages
	c.HTML(http.StatusOK, "chat_detail.html", data)
}

// handlePostChatSend 处理 POST /api/chats/:peer_ref/messages - 发送消息。
func (s *Server) handlePostChatSend(c *gin.Context) {
	peerRef := c.Param("peer_ref")
	if peerRef == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "peer_invalid", "message": "缺少会话引用"})
		return
	}

	// 使用统一的当前账号解析（带 fallback）
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "no_current_account", "message": "请先接入 Telegram 账号"})
		return
	}

	// 解析请求
	var req struct {
		Text         string   `json:"text"`
		Peers        []string `json:"peers"`
		PeerRefs     []string `json:"peer_refs"`
		Recipients   []string `json:"recipients"`
		RecipientIDs []string `json:"recipient_ids"`
		Batch        bool     `json:"batch"`
		Bulk         bool     `json:"bulk"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 尝试 form 解析
		req.Text = c.PostForm("text")
	}

	// 拒绝批量发送参数
	if len(req.Peers) > 0 || len(req.PeerRefs) > 0 || len(req.Recipients) > 0 || len(req.RecipientIDs) > 0 || req.Batch || req.Bulk {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "bulk_not_supported",
			"message": "当前版本仅支持向单个会话发送消息",
		})
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "text_empty", "message": "消息内容不能为空"})
		return
	}
	if len(text) > 4096 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": "text_too_long", "message": "消息内容不能超过 4096 个字符"})
		return
	}

	chatSvc := s.newChatService()

	// 30 秒超时
	sendCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := chatSvc.SendText(sendCtx, selectedID, peerRef, text)
	if err != nil {
		slog.Error("发送消息失败", "error", err, "peer_ref_length", len(peerRef), "text_len", len(text))
		errMsg := s.classifyChatError(err)
		errCode := "telegram_error"
		if chatErr, ok := err.(*chat.ChatError); ok {
			errCode = chatErr.Code
		}
		c.JSON(http.StatusOK, gin.H{"ok": false, "code": errCode, "message": errMsg})
		return
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		Action:       "chat.send_message",
		ResourceType: "peer",
		ResourceID:   peerRef,
		AccountID:    selectedID,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		Metadata: map[string]any{
			"peer_ref": peerRef,
			"text_len": len(text),
		},
		Message: fmt.Sprintf("发送消息到 %s (text_len=%d)", peerRef, len(text)),
	})

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": result,
	})
}

// classifyChatError 分类聊天错误为用户友好消息。
func (s *Server) classifyChatError(err error) string {
	if err == nil {
		return ""
	}

	// ChatError 已经包含用户友好消息
	if chatErr, ok := err.(*chat.ChatError); ok {
		return chatErr.Message
	}

	// 使用 mtproto.ClassifyError 兜底
	errKind := mtproto.ClassifyError(err)
	switch errKind {
	case mtproto.ErrProxyConnectFailed:
		return "无法连接代理，请检查 API 网络代理配置"
	case mtproto.ErrProxyAuthFailed:
		return "代理认证失败，请检查用户名和密码"
	case mtproto.ErrTelegramTimeout:
		return "连接 Telegram 超时，请稍后重试或检查代理"
	case mtproto.ErrSessionInvalid, mtproto.ErrSessionContextLost:
		return "当前账号 Session 已失效，请重新接入"
	case mtproto.ErrUnauthorized:
		return "该 Telegram 账号不可用或已被停用"
	case mtproto.ErrCredentialDisabled:
		return "Telegram API Key 不可用，请检查 API ID / API Hash"
	case mtproto.ErrFloodWait:
		return "Telegram 限制请求过快，请稍后再试"
	case mtproto.ErrTelegramError:
		return "Telegram 返回异常，请稍后重试或检查日志"
	case mtproto.ErrNetworkError:
		return "网络异常，请检查网络连接或代理配置"
	default:
		return "Telegram 返回异常，请稍后重试或检查日志"
	}
}
