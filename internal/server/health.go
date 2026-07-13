package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/telegramclient"
	"github.com/user/atria/internal/version"

	"github.com/gin-gonic/gin"
)

// handleHealthz 返回健康检查和诊断信息。
// 此端点公开访问，无需认证。响应格式向后兼容。
func (s *Server) handleHealthz(c *gin.Context) {
	resp := gin.H{
		"service": "atria",
		"version": version.Short(),
	}

	// 运行时间
	uptime := time.Since(s.startTime)
	resp["uptime"] = formatDuration(uptime)
	resp["uptime_seconds"] = int64(uptime.Seconds())

	// 数据库连通性检查
	dbOK := true
	if err := s.pingDatabase(); err != nil {
		dbOK = false
		resp["database"] = gin.H{"status": "error"}
	} else {
		resp["database"] = gin.H{"status": "ok"}
	}

	// 初始化状态
	initialized := false
	if dbOK {
		initialized = s.adminSvc.IsInitialized()
	}
	resp["initialized"] = initialized

	// 迁移版本
	var migrationVersion int
	if dbOK {
		if err := s.db.Raw("SELECT COALESCE(MAX(version), 0) FROM data_migrations").Scan(&migrationVersion).Error; err != nil {
			migrationVersion = 0
		}
	}
	resp["migration_version"] = migrationVersion

	// 运行时状态摘要
	resp["accounts"] = s.getRuntimeSummary()

	// 总体状态
	if dbOK {
		resp["status"] = "ok"
		c.JSON(http.StatusOK, resp)
	} else {
		resp["status"] = "error"
		c.JSON(http.StatusServiceUnavailable, resp)
	}
}

// pingDatabase 检查数据库连通性。
func (s *Server) pingDatabase() error {
	var result int
	return s.db.Raw("SELECT 1").Scan(&result).Error
}

// getRuntimeSummary 返回账号运行时状态摘要。
func (s *Server) getRuntimeSummary() gin.H {
	summary := gin.H{
		"total": 0, "live": 0, "syncing": 0, "connecting": 0,
		"degraded": 0, "stopped": 0, "offline": 0,
	}

	var accounts []model.TelegramAccount
	if err := s.db.Find(&accounts).Error; err != nil {
		return summary
	}

	summary["total"] = len(accounts)

	if s.runtimeManager == nil {
		summary["stopped"] = len(accounts)
		return summary
	}

	for _, acc := range accounts {
		status := s.runtimeManager.Status(acc.ID)
		switch status.State {
		case telegramclient.RuntimeStateLive:
			summary["live"] = summary["live"].(int) + 1
		case telegramclient.RuntimeStateSyncing:
			summary["syncing"] = summary["syncing"].(int) + 1
		case telegramclient.RuntimeStateConnecting:
			summary["connecting"] = summary["connecting"].(int) + 1
		case telegramclient.RuntimeStateDegraded:
			summary["degraded"] = summary["degraded"].(int) + 1
		case telegramclient.RuntimeStateOffline:
			summary["offline"] = summary["offline"].(int) + 1
		default:
			summary["stopped"] = summary["stopped"].(int) + 1
		}
	}

	return summary
}

// formatDuration 格式化时长为人类可读字符串。
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
}
