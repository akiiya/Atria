// Package server 提供 HTTP 服务器和路由。
package server

import (
	"log/slog"

	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/mtproto"

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
}

// New 创建新的 Server 实例。
func New(cfg *config.Config, db *gorm.DB, key []byte) *Server {
	return &Server{
		cfg:       cfg,
		db:        db,
		key:       key,
		adminSvc:  NewAdminService(db),
		flowStore: mtproto.NewMemoryFlowStore(),
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

	return r.Run(addr)
}
