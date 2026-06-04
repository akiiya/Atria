package server

import (
	"net/http"

	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/update"
	"github.com/user/atria/internal/updater"

	"github.com/gin-gonic/gin"
)

// handleGetSettingsUpdate 处理 GET /settings 页面中的更新信息。
func (s *Server) handleGetSettingsUpdate(c *gin.Context) map[string]any {
	updateSvc := update.NewService(s.db, s.cfg)
	state := updateSvc.GetState()

	return map[string]any{
		"UpdateEnabled":  s.cfg.UpdateEnabled,
		"UpdateStatus":   string(state.Status),
		"UpdateMessage":  state.Message,
		"CurrentVersion": state.CurrentVersion,
		"LatestVersion":  state.LatestVersion,
		"CheckedAt":      state.CheckedAt,
		"DownloadedAt":   state.DownloadedAt,
		"PendingRestart": state.PendingRestart,
		"AssetName":      state.AssetName,
		"IsDocker":       updater.IsDockerEnvironment(),
		"CanApply":       state.Status == updater.StatusDownloaded && !updater.IsDockerEnvironment(),
	}
}

// handlePostUpdateCheck 处理 POST /settings/update/check。
func (s *Server) handlePostUpdateCheck(c *gin.Context) {
	updateSvc := update.NewService(s.db, s.cfg)
	actorID := auth.GetAdminID(c)

	_, err := updateSvc.CheckUpdate(c.Request.Context(), actorID, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = "检查更新失败: " + err.Error()
		data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	c.Redirect(http.StatusFound, "/settings")
}

// handlePostUpdateDownload 处理 POST /settings/update/download。
func (s *Server) handlePostUpdateDownload(c *gin.Context) {
	updateSvc := update.NewService(s.db, s.cfg)
	actorID := auth.GetAdminID(c)

	_, err := updateSvc.DownloadUpdate(c.Request.Context(), actorID, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = "下载更新失败: " + err.Error()
		data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	c.Redirect(http.StatusFound, "/settings")
}

// handlePostUpdateApply 处理 POST /settings/update/apply。
func (s *Server) handlePostUpdateApply(c *gin.Context) {
	// 检查确认字段
	confirm := c.PostForm("confirm")
	if confirm != "apply_update" {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = "缺少确认字段"
		data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	updateSvc := update.NewService(s.db, s.cfg)
	actorID := auth.GetAdminID(c)

	result, err := updateSvc.ApplyUpdate(c.Request.Context(), actorID, c.ClientIP(), c.GetHeader("User-Agent"), false)
	if err != nil {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = "应用更新失败: " + err.Error()
		data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	if !result.Success {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = result.Message
		data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	data := s.newAuthViewData(c, "settings")
	data["Flash"] = result.Message
	data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
	c.HTML(http.StatusOK, "settings.html", data)
}

// handlePostUpdateDryRun 处理 POST /settings/update/dry-run。
func (s *Server) handlePostUpdateDryRun(c *gin.Context) {
	updateSvc := update.NewService(s.db, s.cfg)
	actorID := auth.GetAdminID(c)

	result, err := updateSvc.ApplyUpdate(c.Request.Context(), actorID, c.ClientIP(), c.GetHeader("User-Agent"), true)
	if err != nil {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = "DryRun 失败: " + err.Error()
		data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	data := s.newAuthViewData(c, "settings")
	if result.Success {
		data["Flash"] = "DryRun 验证通过"
	} else {
		data["Error"] = result.Message
	}
	data["UpdateInfo"] = s.handleGetSettingsUpdate(c)
	c.HTML(http.StatusOK, "settings.html", data)
}
