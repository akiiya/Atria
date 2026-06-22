// Package server 提供 HTTP 服务器和路由。
package server

import (
	"log/slog"

	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/mtproto"
	"github.com/user/atria/internal/telegramclient"
	gotdadapter "github.com/user/atria/internal/telegramclient/gotd"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server 持有应用服务器依赖。
type Server struct {
	cfg       *config.Config
	db        *gorm.DB
	key       []byte // AES-256 加密密钥
	adminSvc  *AdminService
	flowStore mtproto.FlowStore // 登录流程存储（请求间共享）

	// Runtime 管理
	runtimeManager *gotdadapter.RuntimeManagerImpl
	eventBus       *telegramclient.EventBus
	accountGate    *gotdadapter.AccountGate
}

// New 创建新的 Server 实例。
func New(cfg *config.Config, db *gorm.DB, key []byte) *Server {
	logger := slog.Default()

	// 创建 EventBus
	bus := telegramclient.NewEventBus(logger)

	// 创建 AccountGate（per-account 执行锁）
	gate := gotdadapter.NewAccountGate()

	// 创建 RuntimeManager，共享同一个 gate
	runtimeMgr := gotdadapter.NewRuntimeManager(db, key, bus, logger)
	runtimeMgr.SetGate(gate)

	// 注入代理 dialer（从数据库读取当前配置）
	// 注意：api_proxy 类型不会创建 dialer，BuildProxyDialerFromDB 会返回明确错误
	if dialer, err := BuildProxyDialerFromDB(db, key); err != nil {
		logger.Warn("Runtime dialer 初始化失败，将使用直连", "error", err)
	} else if dialer != nil {
		runtimeMgr.SetDialer(dialer)
	}

	return &Server{
		cfg:            cfg,
		db:             db,
		key:            key,
		adminSvc:       NewAdminService(db),
		flowStore:      mtproto.NewMemoryFlowStore(),
		runtimeManager: runtimeMgr,
		eventBus:       bus,
		accountGate:    gate,
	}
}

// Run 启动 HTTP 服务器。
func (s *Server) Run() error {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	// 设置路由
	s.setupRoutes(r)

	addr := s.cfg.ListenAddr()
	slog.Info("监听地址", "addr", addr)

	// 服务停止时清理 runtime
	defer func() {
		slog.Info("正在停止所有 AccountRuntime...")
		s.runtimeManager.StopAll()
		s.eventBus.Close()
		slog.Info("所有 AccountRuntime 已停止")
	}()

	return r.Run(addr)
}
