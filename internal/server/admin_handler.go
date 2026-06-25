package server

import (
	"log/slog"
	"net/http"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/credential"

	"github.com/gin-gonic/gin"
)

// csrfCookieName 是 CSRF token 的 Cookie 名称。
const csrfCookieName = "atria_csrf"

// setCSRFToken 生成并设置 CSRF Token。
func (s *Server) setCSRFToken(c *gin.Context) string {
	token, err := auth.GenerateCSRFToken()
	if err != nil {
		slog.Error("生成 CSRF token 失败", "error", err)
		return ""
	}
	c.SetSameSite(s.cfg.CookieSameSiteMode())
	c.SetCookie(csrfCookieName, token, int(s.cfg.SessionTTL.Seconds()), "/", "", s.cfg.CookieSecure, false)
	return token
}

// getCSRFToken 从 Cookie 获取 CSRF Token。
func (s *Server) getCSRFToken(c *gin.Context) string {
	token, _ := c.Cookie(csrfCookieName)
	return token
}

// csrfValidationMiddleware CSRF 校验中间件。
func (s *Server) csrfValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.CSRFEnabled {
			c.Next()
			return
		}

		// 安全方法不需要校验
		switch c.Request.Method {
		case "GET", "HEAD", "OPTIONS":
			c.Next()
			return
		}

		// 从 Header 或 Form 读取 token
		token := c.GetHeader(s.cfg.CSRFHeaderName)
		if token == "" {
			token = c.PostForm(s.cfg.CSRFFieldName)
		}

		// 从 Cookie 获取预期 token
		expected := s.getCSRFToken(c)

		if expected == "" || token == "" || token != expected {
			RenderError(c, http.StatusForbidden, "CSRF 校验失败", "请求无效，请刷新页面重试")
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetInit 处理 GET /init。
func (s *Server) handleGetInit(c *gin.Context) {
	if s.adminSvc.IsInitialized() {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	data := NewViewData(s.cfg, "init")
	data.CSRFToken = s.setCSRFToken(c)
	c.HTML(http.StatusOK, "init_page", data.ToMap())
}

// PostInit 处理 POST /init。
func (s *Server) handlePostInit(c *gin.Context) {
	if s.adminSvc.IsInitialized() {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	username := c.PostForm("username")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	// API Key 字段（可选）
	apiDisplayName := c.PostForm("api_display_name")
	apiIDStr := c.PostForm("api_id")
	apiHash := c.PostForm("api_hash")

	if password != confirmPassword {
		data := NewViewData(s.cfg, "init")
		data.CSRFToken = s.setCSRFToken(c)
		data.Error = "两次输入的密码不一致"
		c.HTML(http.StatusOK, "init_page", data.ToMap())
		return
	}

	admin, err := s.adminSvc.Initialize(InitializeInput{
		Username:       username,
		Password:       password,
		APIDisplayName: apiDisplayName,
		APIID:          apiIDStr,
		APIHash:        apiHash,
	})
	if err != nil {
		data := NewViewData(s.cfg, "init")
		data.CSRFToken = s.setCSRFToken(c)
		data.Error = err.Error()
		c.HTML(http.StatusOK, "init_page", data.ToMap())
		return
	}

	// 如果提供了 API Key，创建默认凭据
	if apiIDStr != "" && apiHash != "" {
		credSvc := credential.NewService(s.db, s.key)
		displayName := apiDisplayName
		if displayName == "" {
			displayName = "Default API"
		}
		_, credErr := credSvc.Create(credential.CreateInput{
			DisplayName: displayName,
			APIID:       apiIDStr,
			APIHash:     apiHash,
			Status:      "enabled",
			RiskPolicy:  "disabled",
		})
		if credErr != nil {
			slog.Error("初始化时创建默认 API Key 失败", "error", credErr)
			// 不阻断初始化流程，管理员可以后续在设置中配置
		}
	}

	// 审计日志
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      "0",
		Action:       "admin.initialized",
		ResourceType: "admin",
		ResourceID:   "0",
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "管理员初始化成功",
	})

	// 自动登录
	s.setSessionCookie(c, admin.ID, admin.Username)
	c.Redirect(http.StatusFound, "/app/#/dashboard")
}

// GetLogin 处理 GET /login。
func (s *Server) handleGetLogin(c *gin.Context) {
	if !s.adminSvc.IsInitialized() {
		c.Redirect(http.StatusFound, "/init")
		return
	}

	if s.isLoggedIn(c) {
		c.Redirect(http.StatusFound, "/app/#/dashboard")
		return
	}

	data := NewViewData(s.cfg, "login")
	data.CSRFToken = s.setCSRFToken(c)

	// 检查 flash 参数
	flash := c.Query("flash")
	if flash != "" {
		data.Flash = flash
	}

	c.HTML(http.StatusOK, "login_page", data.ToMap())
}

// PostLogin 处理 POST /login。
func (s *Server) handlePostLogin(c *gin.Context) {
	if !s.adminSvc.IsInitialized() {
		c.Redirect(http.StatusFound, "/init")
		return
	}

	username := c.PostForm("username")
	password := c.PostForm("password")

	admin, err := s.adminSvc.Login(username, password)
	if err != nil {
		// 登录失败审计
		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "anonymous",
			ActorID:      "0",
			Action:       "admin.login_failed",
			ResourceType: "admin",
			ResourceID:   "0",
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "登录失败",
		})

		data := NewViewData(s.cfg, "login")
		data.CSRFToken = s.setCSRFToken(c)
		data.Error = "用户名或密码不正确"
		c.HTML(http.StatusOK, "login_page", data.ToMap())
		return
	}

	// 登录成功审计
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      "0",
		Action:       "admin.login_success",
		ResourceType: "admin",
		ResourceID:   "0",
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "管理员登录成功",
	})

	s.setSessionCookie(c, admin.ID, admin.Username)
	c.Redirect(http.StatusFound, "/app/#/dashboard")
}

// PostLogout 处理 POST /logout。
func (s *Server) handlePostLogout(c *gin.Context) {
	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      "0",
		Action:       "admin.logout",
		ResourceType: "admin",
		ResourceID:   "0",
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "管理员登出",
	})

	s.clearSessionCookie(c)
	c.Redirect(http.StatusFound, "/login")
}

// PostChangePassword 处理 POST /settings/password。
func (s *Server) handlePostChangePassword(c *gin.Context) {
	adminID := auth.GetAdminID(c)
	if adminID == 0 {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_new_password")

	if newPassword != confirmPassword {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = "两次输入的新密码不一致"
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	err := s.adminSvc.ChangePassword(adminID, currentPassword, newPassword)
	if err != nil {
		data := s.newAuthViewData(c, "settings")
		data["Error"] = err.Error()
		c.HTML(http.StatusOK, "settings.html", data)
		return
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      "0",
		Action:       "admin.password_changed",
		ResourceType: "admin",
		ResourceID:   "0",
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "管理员密码已修改",
	})

	// 清除 Session，要求重新登录
	s.clearSessionCookie(c)
	c.Redirect(http.StatusFound, "/login?flash=password_changed")
}

// setSessionCookie 设置 Session Cookie。
func (s *Server) setSessionCookie(c *gin.Context, adminID uint, username string) {
	claims := auth.NewSessionClaims(adminID, username, s.cfg.SessionTTL)
	token, err := auth.EncodeSessionToken(s.key, claims)
	if err != nil {
		slog.Error("编码 session token 失败", "error", err)
		return
	}

	c.SetSameSite(s.cfg.CookieSameSiteMode())
	c.SetCookie(
		s.cfg.CookieName,
		token,
		int(s.cfg.SessionTTL.Seconds()),
		"/",
		"",
		s.cfg.CookieSecure,
		true, // HttpOnly
	)
}

// clearSessionCookie 清除 Session Cookie 和 CSRF Cookie。
func (s *Server) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(s.cfg.CookieSameSiteMode())
	c.SetCookie(s.cfg.CookieName, "", -1, "/", "", s.cfg.CookieSecure, true)
	c.SetCookie(csrfCookieName, "", -1, "/", "", s.cfg.CookieSecure, false)
}

// isLoggedIn 检查当前请求是否已登录。
func (s *Server) isLoggedIn(c *gin.Context) bool {
	token, err := c.Cookie(s.cfg.CookieName)
	if err != nil || token == "" {
		return false
	}
	_, err = auth.DecodeSessionToken(s.key, token)
	return err == nil
}
