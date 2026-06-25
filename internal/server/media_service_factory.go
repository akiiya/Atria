package server

import (
	"log/slog"

	"github.com/user/atria/internal/media"
	gotdadapter "github.com/user/atria/internal/telegramclient/gotd"
)

func (s *Server) newMediaService() *media.Service {
	adapter := gotdadapter.NewAdapter(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	adapter.SetGate(s.accountGate)
	adapter.SetRuntime(s.runtimeManager)
	if dialer, _ := BuildProxyDialerFromDB(s.db, s.key); dialer != nil {
		adapter.SetDialer(dialer)
	}
	return media.NewService(s.db, adapter, s.cfg.DataDir, slog.Default())
}
