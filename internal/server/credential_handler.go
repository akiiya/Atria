package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/model"

	"github.com/gin-gonic/gin"
)

// GetCredentials 处理 GET /credentials - 凭据列表。
func (s *Server) handleGetCredentials(c *gin.Context) {
	credSvc := credential.NewService(s.db, s.key)
	credentials, err := credSvc.List()
	if err != nil {
		slog.Error("查询凭据列表失败", "error", err)
		RenderError(c, http.StatusInternalServerError, "服务器错误", "查询凭据失败")
		return
	}

	data := s.newCredentialViewData(c, "credentials")
	data["Credentials"] = credentials
	c.HTML(http.StatusOK, "credentials.html", data)
}

// GetCredentialNew 处理 GET /credentials/new - 新增凭据页面。
func (s *Server) handleGetCredentialNew(c *gin.Context) {
	data := s.newCredentialViewData(c, "credentials")
	data["FormAction"] = "/credentials"
	data["IsEdit"] = false
	c.HTML(http.StatusOK, "credential_form.html", data)
}

// PostCredential 处理 POST /credentials - 创建凭据。
func (s *Server) handlePostCredential(c *gin.Context) {
	credSvc := credential.NewService(s.db, s.key)

	input := credential.CreateInput{
		DisplayName: c.PostForm("display_name"),
		APIID:       c.PostForm("api_id"),
		APIHash:     c.PostForm("api_hash"),
		Status:      c.PostForm("status"),
		RiskPolicy:  c.PostForm("risk_policy"),
	}

	cred, err := credSvc.Create(input)
	if err != nil {
		data := s.newCredentialViewData(c, "credentials")
		data["Error"] = err.Error()
		data["FormInput"] = input
		data["FormAction"] = "/credentials"
		data["IsEdit"] = false
		c.HTML(http.StatusOK, "credential_form.html", data)
		return
	}

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
		Action:       "api_credential.created",
		ResourceType: "api_credential",
		ResourceID:   fmt.Sprintf("%d", cred.ID),
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      fmt.Sprintf("创建 API 凭据: %s", cred.DisplayName),
		Metadata: map[string]any{
			"display_name": cred.DisplayName,
			"api_id":       cred.APIID,
			"status":       cred.Status,
			"risk_policy":  cred.RiskPolicy,
		},
	})

	c.Redirect(http.StatusFound, "/credentials")
}

// GetCredentialEdit 处理 GET /credentials/:id/edit - 编辑凭据页面。
func (s *Server) handleGetCredentialEdit(c *gin.Context) {
	id, err := parseCredentialID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "凭据 ID 不合法")
		return
	}

	credSvc := credential.NewService(s.db, s.key)
	cred, err := credSvc.GetByID(id)
	if err != nil {
		RenderError(c, http.StatusNotFound, "未找到", "凭据不存在")
		return
	}

	data := s.newCredentialViewData(c, "credentials")
	data["Credential"] = cred
	data["FormAction"] = fmt.Sprintf("/credentials/%d", id)
	data["IsEdit"] = true
	data["APIHashHint"] = cred.APIHashHint
	c.HTML(http.StatusOK, "credential_form.html", data)
}

// PostCredentialUpdate 处理 POST /credentials/:id - 更新凭据。
func (s *Server) handlePostCredentialUpdate(c *gin.Context) {
	id, err := parseCredentialID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "凭据 ID 不合法")
		return
	}

	credSvc := credential.NewService(s.db, s.key)

	input := credential.UpdateInput{
		DisplayName: c.PostForm("display_name"),
		APIID:       c.PostForm("api_id"),
		APIHash:     c.PostForm("api_hash"),
		Status:      c.PostForm("status"),
		RiskPolicy:  c.PostForm("risk_policy"),
	}

	cred, err := credSvc.Update(id, input)
	if err != nil {
		cred, _ := credSvc.GetByID(id)
		data := s.newCredentialViewData(c, "credentials")
		data["Error"] = err.Error()
		data["Credential"] = cred
		data["FormInput"] = input
		data["FormAction"] = fmt.Sprintf("/credentials/%d", id)
		data["IsEdit"] = true
		if cred != nil {
			data["APIHashHint"] = cred.APIHashHint
		}
		c.HTML(http.StatusOK, "credential_form.html", data)
		return
	}

	// 审计日志
	metadata := map[string]any{
		"display_name": cred.DisplayName,
		"api_id":       cred.APIID,
		"status":       cred.Status,
		"risk_policy":  cred.RiskPolicy,
	}
	if input.APIHash != "" {
		metadata["hash_rotated"] = true
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
		Action:       "api_credential.updated",
		ResourceType: "api_credential",
		ResourceID:   fmt.Sprintf("%d", cred.ID),
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      fmt.Sprintf("更新 API 凭据: %s", cred.DisplayName),
		Metadata:     metadata,
	})

	c.Redirect(http.StatusFound, "/credentials")
}

// PostCredentialStatus 处理 POST /credentials/:id/status - 启用/禁用凭据。
func (s *Server) handlePostCredentialStatus(c *gin.Context) {
	id, err := parseCredentialID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "凭据 ID 不合法")
		return
	}

	status := c.PostForm("status")
	credSvc := credential.NewService(s.db, s.key)

	if err := credSvc.UpdateStatus(id, status); err != nil {
		RenderError(c, http.StatusBadRequest, "操作失败", err.Error())
		return
	}

	// 如果禁用的是当前选中凭据，清除当前选择
	credID := auth.GetCredentialID(c)
	if credID == id && status == string(model.APICredentialStatusDisabled) {
		s.clearCurrentCredential(c)
	}

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
		Action:       "api_credential.status_changed",
		ResourceType: "api_credential",
		ResourceID:   fmt.Sprintf("%d", id),
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      fmt.Sprintf("凭据状态变更为: %s", status),
		Metadata: map[string]any{
			"new_status": status,
		},
	})

	c.Redirect(http.StatusFound, "/credentials")
}

// PostCredentialDelete 处理 POST /credentials/:id/delete - 删除凭据。
func (s *Server) handlePostCredentialDelete(c *gin.Context) {
	id, err := parseCredentialID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "凭据 ID 不合法")
		return
	}

	credSvc := credential.NewService(s.db, s.key)

	// 获取凭据信息用于审计
	cred, err := credSvc.GetByID(id)
	if err != nil {
		RenderError(c, http.StatusNotFound, "未找到", "凭据不存在")
		return
	}

	if err := credSvc.Delete(id); err != nil {
		data := s.newCredentialViewData(c, "credentials")
		data["Error"] = err.Error()
		credentials, _ := credSvc.List()
		data["Credentials"] = credentials
		c.HTML(http.StatusOK, "credentials.html", data)
		return
	}

	// 如果删除的是当前选中凭据，清除当前选择
	credID := auth.GetCredentialID(c)
	if credID == id {
		s.clearCurrentCredential(c)
	}

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
		Action:       "api_credential.deleted",
		ResourceType: "api_credential",
		ResourceID:   fmt.Sprintf("%d", id),
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      fmt.Sprintf("删除 API 凭据: %s", cred.DisplayName),
		Metadata: map[string]any{
			"display_name": cred.DisplayName,
			"api_id":       cred.APIID,
		},
	})

	c.Redirect(http.StatusFound, "/credentials")
}

// PostCredentialSelect 处理 POST /credentials/select - 切换当前凭据。
func (s *Server) handlePostCredentialSelect(c *gin.Context) {
	credIDStr := c.PostForm("credential_id")

	// 空值表示清除选择
	if credIDStr == "" {
		s.clearCurrentCredential(c)
		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
			Action:       "api_credential.selected",
			ResourceType: "api_credential",
			ResourceID:   "0",
			RiskLevel:    "low",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "清除当前凭据选择",
		})
		redirectBack(c)
		return
	}

	credID, err := strconv.ParseUint(credIDStr, 10, 32)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "凭据 ID 不合法")
		return
	}

	credSvc := credential.NewService(s.db, s.key)

	// 验证凭据有效
	if !credSvc.IsValidCredential(uint(credID)) {
		RenderError(c, http.StatusBadRequest, "操作失败", "凭据不存在或已禁用")
		return
	}

	// 更新 Session
	s.setCurrentCredential(c, uint(credID))

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
		Action:       "api_credential.selected",
		ResourceType: "api_credential",
		ResourceID:   credIDStr,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      fmt.Sprintf("选择 API 凭据 ID=%s", credIDStr),
	})

	redirectBack(c)
}

// setCurrentCredential 设置当前凭据到 Session。
func (s *Server) setCurrentCredential(c *gin.Context, credID uint) {
	token, _ := c.Cookie(s.cfg.CookieName)
	if token == "" {
		return
	}

	claims, err := auth.DecodeSessionToken(s.key, token)
	if err != nil {
		return
	}

	claims.CurrentCredentialID = credID

	newToken, err := auth.EncodeSessionToken(s.key, claims)
	if err != nil {
		slog.Error("编码 session token 失败", "error", err)
		return
	}

	c.SetCookie(
		s.cfg.CookieName,
		newToken,
		int(s.cfg.SessionTTL.Seconds()),
		"/",
		"",
		s.cfg.CookieSecure,
		true,
	)
}

// clearCurrentCredential 清除当前凭据选择。
func (s *Server) clearCurrentCredential(c *gin.Context) {
	s.setCurrentCredential(c, 0)
}

// newCredentialViewData 创建带凭据切换器数据的 ViewData。
func (s *Server) newCredentialViewData(c *gin.Context, activeNav string) map[string]any {
	data := NewViewData(s.cfg, activeNav)
	data.IsInitialized = true
	data.IsAuthenticated = true
	data.CurrentAdminUsername = auth.GetUsername(c)

	// 生成 CSRF token
	token := s.setCSRFToken(c)
	data.CSRFToken = token

	// 获取当前凭据 ID
	credID := auth.GetCredentialID(c)

	// 获取凭据切换器列表
	credSvc := credential.NewService(s.db, s.key)
	switcherItems, err := credSvc.GetSwitcherItems(credID)
	if err == nil {
		data.CredentialsForSwitcher = switcherItems
	}

	// 设置当前凭据信息
	if credID > 0 {
		cred, err := credSvc.GetByID(credID)
		if err == nil && cred.Status == model.APICredentialStatusEnabled {
			data.CurrentCredentialID = credID
			data.CurrentCredentialName = cred.DisplayName
		}
	}

	return data.ToMap()
}

// parseCredentialID 从 URL 参数解析凭据 ID。
func parseCredentialID(c *gin.Context) (uint, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("ID 不合法")
	}
	return uint(id), nil
}

// redirectBack 重定向回 Referer 或首页。
func redirectBack(c *gin.Context) {
	referer := c.GetHeader("Referer")
	if referer != "" {
		c.Redirect(http.StatusFound, referer)
	} else {
		c.Redirect(http.StatusFound, "/")
	}
}
