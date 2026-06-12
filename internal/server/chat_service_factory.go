package server

import (
	"log/slog"

	"github.com/user/atria/internal/chat"
	gotdadapter "github.com/user/atria/internal/telegramclient/gotd"
)

func (s *Server) newChatService() *chat.ChatService {
	adapter := gotdadapter.NewAdapter(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	adapter.SetGate(s.accountGate)
	adapter.SetRuntime(s.runtimeManager)
	if dialer, _ := BuildProxyDialerFromDB(s.db, s.key); dialer != nil {
		adapter.SetDialer(dialer)
	}
	return chat.NewChatService(s.db, s.key, adapter, slog.Default())
}
