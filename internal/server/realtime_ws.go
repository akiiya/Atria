package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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

	// Origin 校验
	origin := c.GetHeader("Origin")
	if origin != "" && !isSameOrigin(origin, c.Request.Host) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "code": "forbidden", "message": "Origin 不允许"})
		return
	}

	// 升级为 WebSocket
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
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

	// 对于 message 事件，过滤敏感字段
	switch event.Type {
	case telegramclient.EventMessageNew, telegramclient.EventMessageEdited:
		if msg, ok := event.Payload.(telegramclient.Message); ok {
			return map[string]interface{}{
				"id":                  msg.ID,
				"telegram_message_id": msg.TelegramMessageID,
				"peer_ref":            msg.PeerRef,
				"direction":           string(msg.Direction),
				"sender_name":         msg.SenderName,
				"text":                msg.Text,
				"kind":                string(msg.Kind),
				"caption":             msg.Caption,
				"sent_at":             msg.SentAt.UTC().Format(time.RFC3339),
				"is_outgoing":         msg.IsOutgoing,
				"status":              string(msg.Status),
			}
		}

	case telegramclient.EventMessageDeleted:
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			return payload
		}

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
	}

	// 对于 sync/status 事件，直接返回 payload
	return event.Payload
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
