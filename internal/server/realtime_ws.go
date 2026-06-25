package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/telegramclient"

	"github.com/gin-gonic/gin"
	"nhooyr.io/websocket"
)

// realtimeWSEnvelope WebSocket 推送消息的统一 envelope。
type realtimeWSEnvelope struct {
	Type      telegramclient.UpdateEventType `json:"type"`
	EventID   string                         `json:"event_id"`
	AccountID uint                           `json:"account_id"`
	PeerRef   string                         `json:"peer_ref,omitempty"`
	CreatedAt string                         `json:"created_at"`
	Payload   interface{}                    `json:"payload,omitempty"`
}

// wsUpdateSink 实现 telegramclient.UpdateSink，将事件发送到 WebSocket。
type wsUpdateSink struct {
	ch     chan telegramclient.UpdateEvent
	logger *slog.Logger
}

func (s *wsUpdateSink) Send(event telegramclient.UpdateEvent) error {
	select {
	case s.ch <- event:
		return nil
	default:
		// channel 满，丢弃事件
		s.logger.Warn("WebSocket sink channel 满，丢弃事件",
			"event_type", event.Type,
			"account_id", event.AccountID,
		)
		return nil
	}
}

// handleRealtimeWS 处理 WebSocket 连接。
// GET /api/realtime/ws
func (s *Server) handleRealtimeWS(c *gin.Context) {
	// 鉴权检查
	username := auth.GetUsername(c)
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "code": "unauthorized"})
		return
	}

	// 检查 selected account
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "no_current_account", "message": "请先接入 Telegram 账号"})
		return
	}

	// Origin 校验：无 Origin header 或不匹配则拒绝
	origin := c.GetHeader("Origin")
	if origin == "" || !isSameOrigin(origin, c.Request.Host) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "code": "forbidden", "message": "Origin 不允许"})
		return
	}

	// 升级为 WebSocket
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		OriginPatterns: []string{c.Request.Host},
	})
	if err != nil {
		slog.Error("WebSocket 升级失败", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	slog.Info("WebSocket 连接建立", "account_id", selectedID)

	// 创建 event sink
	sink := &wsUpdateSink{
		ch:     make(chan telegramclient.UpdateEvent, 64),
		logger: slog.Default(),
	}

	// 订阅 EventBus
	sub, err := s.eventBus.Subscribe(selectedID, sink)
	if err != nil {
		slog.Error("WebSocket 订阅 EventBus 失败", "error", err, "account_id", selectedID)
		conn.Close(websocket.StatusInternalError, "订阅失败")
		return
	}
	defer sub.Close()

	// 发送 hello 事件
	status := s.runtimeManager.Status(selectedID)
	hello := realtimeWSEnvelope{
		Type:      telegramclient.EventAccountConnected,
		EventID:   fmt.Sprintf("hello_%d_%d", selectedID, time.Now().UnixNano()),
		AccountID: selectedID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"state": string(status.State),
		},
	}
	if err := writeJSON(conn, hello); err != nil {
		slog.Warn("WebSocket 写入 hello 失败", "error", err)
		return
	}

	// 心跳 ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	ctx := context.Background()

	for {
		select {
		case event := <-sink.ch:
			// 过滤：只推送当前 account 的事件
			if event.AccountID != selectedID {
				continue
			}

			envelope := realtimeWSEnvelope{
				Type:      event.Type,
				EventID:   event.EventID,
				AccountID: event.AccountID,
				PeerRef:   event.PeerRef,
				CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339),
				Payload:   sanitizePayload(event),
			}

			if err := writeJSON(conn, envelope); err != nil {
				slog.Warn("WebSocket 写入事件失败", "error", err, "event_type", event.Type)
				return
			}

		case <-pingTicker.C:
			// 发送 ping
			if err := conn.Ping(ctx); err != nil {
				slog.Warn("WebSocket ping 失败", "error", err)
				return
			}
		}
	}
}

// sanitizePayload 过滤敏感字段，只返回安全的 payload。
func sanitizePayload(event telegramclient.UpdateEvent) interface{} {
	if event.Payload == nil {
		return nil
	}

	switch event.Type {
	case telegramclient.EventMessageNew, telegramclient.EventMessageEdited:
		if msg, ok := event.Payload.(telegramclient.Message); ok {
			return sanitizeMessageDTO(msg)
		}
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			return sanitizeMessageMap(payload, event.PeerRef)
		}

	case telegramclient.EventMessageDeleted:
		return sanitizeDeletedPayload(event)

	case telegramclient.EventDialogUpserted:
		if dlg, ok := event.Payload.(telegramclient.Dialog); ok {
			return map[string]interface{}{
				"peer_ref":             dlg.PeerRef,
				"peer_type":            string(dlg.PeerType),
				"title":                dlg.Title,
				"username":             dlg.Username,
				"avatar_text":          dlg.AvatarText,
				"last_message_preview": dlg.LastMessagePreview,
				"last_message_at":      dlg.LastMessageAt.UTC().Format(time.RFC3339),
				"unread_count":         dlg.UnreadCount,
				"is_pinned":            dlg.IsPinned,
				"is_muted":             dlg.IsMuted,
			}
		}
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			return sanitizeDialogMap(payload, event.PeerRef)
		}
	}

	return sanitizeGenericPayload(event.Payload)
}

func sanitizeMessageDTO(msg telegramclient.Message) map[string]interface{} {
	messageID := msg.TelegramMessageID
	if messageID == 0 {
		if parsed, err := strconv.Atoi(msg.ID); err == nil {
			messageID = parsed
		}
	}
	peerRef := msg.PeerRef
	result := map[string]interface{}{
		"id":                  messageID,
		"telegram_message_id": messageID,
		"peer_ref":            peerRef,
		"direction":           string(msg.Direction),
		"sender_name":         msg.SenderName,
		"text":                msg.Text,
		"kind":                string(msg.Kind),
		"message_type":        string(msg.Kind),
		"caption":             msg.Caption,
		"sent_at":             msg.SentAt.UTC().Format(time.RFC3339),
		"is_outgoing":         msg.IsOutgoing,
		"status":              string(msg.Status),
	}
	if msg.Media != nil {
		result["media"] = msg.Media
	}
	return result
}

func sanitizeMessageMap(payload map[string]interface{}, fallbackPeerRef string) map[string]interface{} {
	allowed := map[string]bool{
		"id":                  true,
		"telegram_message_id": true,
		"local_id":            true,
		"client_pending_id":   true,
		"peer_ref":            true,
		"direction":           true,
		"sender_name":         true,
		"text":                true,
		"kind":                true,
		"message_type":        true,
		"caption":             true,
		"sent_at":             true,
		"is_outgoing":         true,
		"status":              true,
		"pending":             true,
		"media":               true,
	}
	result := copyAllowedPayloadFields(payload, allowed)
	if fallbackPeerRef != "" {
		if _, ok := result["peer_ref"]; !ok {
			result["peer_ref"] = fallbackPeerRef
		}
	}
	if _, ok := result["telegram_message_id"]; !ok {
		if id, ok := payloadInt(payload["id"]); ok {
			result["telegram_message_id"] = id
			result["id"] = id
		}
	} else if id, ok := payloadInt(result["telegram_message_id"]); ok {
		result["telegram_message_id"] = id
		result["id"] = id
	}
	if _, ok := result["message_type"]; !ok {
		if kind, ok := result["kind"]; ok {
			result["message_type"] = kind
		}
	}
	return result
}

func sanitizeDeletedPayload(event telegramclient.UpdateEvent) map[string]interface{} {
	result := map[string]interface{}{}
	if event.PeerRef != "" {
		result["peer_ref"] = event.PeerRef
	}
	payload, _ := event.Payload.(map[string]interface{})
	if payload == nil {
		result["telegram_message_ids"] = []int{}
		return result
	}

	ids := payloadIntSlice(payload["telegram_message_ids"])
	if len(ids) == 0 {
		ids = payloadIntSlice(payload["message_ids"])
	}
	if len(ids) == 0 {
		if id, ok := payloadInt(payload["telegram_message_id"]); ok {
			ids = []int{id}
		}
	}
	if len(ids) == 0 {
		if id, ok := payloadInt(payload["id"]); ok {
			ids = []int{id}
		}
	}
	if peerRef, ok := payload["peer_ref"].(string); ok && peerRef != "" {
		result["peer_ref"] = peerRef
	}
	result["telegram_message_ids"] = ids
	return result
}

func sanitizeDialogMap(payload map[string]interface{}, fallbackPeerRef string) map[string]interface{} {
	allowed := map[string]bool{
		"peer_ref":             true,
		"peer_type":            true,
		"title":                true,
		"username":             true,
		"avatar_text":          true,
		"avatar_placeholder":   true,
		"last_message_preview": true,
		"last_message_at":      true,
		"unread_count":         true,
		"is_pinned":            true,
		"is_muted":             true,
	}
	result := copyAllowedPayloadFields(payload, allowed)
	if fallbackPeerRef != "" {
		if _, ok := result["peer_ref"]; !ok {
			result["peer_ref"] = fallbackPeerRef
		}
	}
	return result
}

func sanitizeGenericPayload(payload interface{}) interface{} {
	switch p := payload.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(p))
		for k, v := range p {
			if isSensitivePayloadKey(k) {
				continue
			}
			result[k] = sanitizeGenericPayload(v)
		}
		return result
	case []interface{}:
		items := make([]interface{}, 0, len(p))
		for _, item := range p {
			items = append(items, sanitizeGenericPayload(item))
		}
		return items
	default:
		return payload
	}
}

func copyAllowedPayloadFields(payload map[string]interface{}, allowed map[string]bool) map[string]interface{} {
	result := make(map[string]interface{}, len(allowed))
	for k, v := range payload {
		if allowed[k] && !isSensitivePayloadKey(k) {
			result[k] = sanitizeGenericPayload(v)
		}
	}
	return result
}

func isSensitivePayloadKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "access_hash") ||
		strings.Contains(k, "api_hash") ||
		strings.Contains(k, "proxy_password") ||
		strings.Contains(k, "session_path") ||
		strings.Contains(k, "session_file") ||
		strings.Contains(k, "phone") ||
		strings.Contains(k, "message_body")
}

func payloadIntSlice(v interface{}) []int {
	switch ids := v.(type) {
	case []int:
		return ids
	case []int64:
		result := make([]int, 0, len(ids))
		for _, id := range ids {
			result = append(result, int(id))
		}
		return result
	case []float64:
		result := make([]int, 0, len(ids))
		for _, id := range ids {
			result = append(result, int(id))
		}
		return result
	case []interface{}:
		result := make([]int, 0, len(ids))
		for _, raw := range ids {
			if id, ok := payloadInt(raw); ok {
				result = append(result, id)
			}
		}
		return result
	default:
		if id, ok := payloadInt(v); ok {
			return []int{id}
		}
		return nil
	}
}

func payloadInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	default:
		return 0, false
	}
}

// writeJSON 写入 JSON 到 WebSocket，带写超时。
func writeJSON(conn *websocket.Conn, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("JSON 序列化失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return conn.Write(ctx, websocket.MessageText, data)
}

// isSameOrigin 检查 Origin 是否与 Host 同源。
func isSameOrigin(origin, host string) bool {
	if origin == "" || host == "" {
		return false
	}

	// 从 origin 中提取 host
	originHost := origin
	if strings.HasPrefix(origin, "http://") {
		originHost = strings.TrimPrefix(origin, "http://")
	} else if strings.HasPrefix(origin, "https://") {
		originHost = strings.TrimPrefix(origin, "https://")
	}

	// 去掉端口比较
	originHost = strings.Split(originHost, ":")[0]
	hostName := strings.Split(host, ":")[0]

	return originHost == hostName
}

// ===== Dev/Test Event Publish =====

// devPublishAllowedEvents 白名单事件类型。
var devPublishAllowedEvents = map[telegramclient.UpdateEventType]bool{
	telegramclient.EventMessageNew:     true,
	telegramclient.EventMessageEdited:  true,
	telegramclient.EventMessageDeleted: true,
	telegramclient.EventDialogUpserted: true,
	telegramclient.EventSyncStarted:    true,
	telegramclient.EventSyncDone:       true,
	telegramclient.EventSyncFailed:     true,
}

// devRealtimeMiddleware 检查 ATRIA_DEV_REALTIME_TEST 环境变量。
func devRealtimeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("ATRIA_DEV_REALTIME_TEST") != "1" {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "code": "not_found"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// handleRealtimeDevPublish 处理 dev/test 事件注入。
// POST /api/realtime/dev/publish
// 默认关闭，仅当 ATRIA_DEV_REALTIME_TEST=1 时可用。
func (s *Server) handleRealtimeDevPublish(c *gin.Context) {

	// 鉴权
	username := auth.GetUsername(c)
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "code": "unauthorized"})
		return
	}

	// 检查 selected account
	selectedID := s.resolveCurrentAccountID(c)
	if selectedID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "no_current_account"})
		return
	}

	// 解析请求
	var req struct {
		Type    telegramclient.UpdateEventType `json:"type"`
		PeerRef string                         `json:"peer_ref,omitempty"`
		Payload interface{}                    `json:"payload,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "code": "invalid_request", "message": err.Error()})
		return
	}

	// 白名单检查
	if !devPublishAllowedEvents[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"code":    "event_type_not_allowed",
			"message": "不允许的事件类型",
		})
		return
	}

	// 构造事件
	event := telegramclient.UpdateEvent{
		EventID:   fmt.Sprintf("dev_%d_%d", selectedID, time.Now().UnixNano()),
		AccountID: selectedID,
		Type:      req.Type,
		PeerRef:   req.PeerRef,
		Payload:   req.Payload,
		CreatedAt: time.Now(),
	}
	event.Payload = sanitizePayload(event)

	// 发布到 EventBus
	s.eventBus.Publish(selectedID, event)

	slog.Info("Dev 事件已发布",
		"event_type", req.Type,
		"account_id", selectedID,
		"peer_ref", req.PeerRef,
	)

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"event_id": event.EventID,
	})
}
