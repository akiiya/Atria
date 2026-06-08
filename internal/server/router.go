package server

import (
	"net/http"

	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/version"
	"github.com/user/atria/internal/web"

	"github.com/gin-gonic/gin"
)

func (s *Server) setupRoutes(r *gin.Engine) {
	// 解析嵌入的模板
	pageTmpls, err := web.ParseTemplates()
	if err != nil {
		panic("解析嵌入模板失败: " + err.Error())
	}

	// 设置模板
	r.SetHTMLTemplate(pageTmpls.Template)

	// 提供嵌入的静态文件
	staticFS, err := web.Static()
	if err != nil {
		panic("加载嵌入静态文件失败: " + err.Error())
	}
	r.StaticFS("/static", http.FS(staticFS))

	// CSRF 中间件
	csrfMiddleware := s.csrfValidationMiddleware()

	// 认证中间件
	authMiddleware := auth.RequireAuth(s.key, s.cfg.CookieName)

	// 健康检查（公开）
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "atria",
			"version": version.Short(),
		})
	})

	// ===== 公开路由 =====

	// 初始化页面
	r.GET("/init", func(c *gin.Context) {
		s.handleGetInit(c)
	})
	r.POST("/init", csrfMiddleware, func(c *gin.Context) {
		s.handlePostInit(c)
	})

	// 登录页面
	r.GET("/login", func(c *gin.Context) {
		s.handleGetLogin(c)
	})
	r.POST("/login", csrfMiddleware, func(c *gin.Context) {
		s.handlePostLogin(c)
	})

	// ===== 受保护路由 =====

	// 登出
	r.POST("/logout", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostLogout(c)
	})

	// 仪表盘
	r.GET("/", authMiddleware, func(c *gin.Context) {
		data := s.newAuthViewData(c, "dashboard")
		c.HTML(http.StatusOK, "index.html", data)
	})

	// 系统设置
	r.GET("/settings", authMiddleware, func(c *gin.Context) {
		s.handleGetSettings(c)
	})

	// 修改密码
	r.POST("/settings/password", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostChangePassword(c)
	})

	// 保存 API Key 配置
	r.POST("/settings/api-key", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostSettingsAPIKey(c)
	})

	// 保存代理配置
	r.POST("/settings/proxy", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostSettingsProxy(c)
	})

	// 代理检测
	r.POST("/settings/proxy/test", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostProxyTest(c)
	})

	// 更新操作（JSON 接口，用于局部交互）
	r.POST("/settings/update/check/json", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateCheckJSON(c)
	})
	r.POST("/settings/update/download/json", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateDownloadJSON(c)
	})
	r.POST("/settings/update/apply/json", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateApplyJSON(c)
	})
	r.POST("/settings/update/dry-run/json", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateDryRunJSON(c)
	})

	// 更新操作（保留传统表单提交作为降级）
	r.POST("/settings/update/check", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateCheck(c)
	})
	r.POST("/settings/update/download", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateDownload(c)
	})
	r.POST("/settings/update/apply", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateApply(c)
	})
	r.POST("/settings/update/dry-run", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostUpdateDryRun(c)
	})

	// 账号 Session 切换
	r.POST("/accounts/select", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountSelect(c)
	})

	// ===== API 凭据路由（重定向到系统设置） =====

	// 凭据列表 - 重定向到系统设置
	r.GET("/credentials", authMiddleware, func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/settings#telegram-api-key")
	})

	// 新增凭据页面 - 重定向到系统设置
	r.GET("/credentials/new", authMiddleware, func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/settings#telegram-api-key")
	})

	// 创建凭据
	r.POST("/credentials", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostCredential(c)
	})

	// 编辑凭据页面
	r.GET("/credentials/:id/edit", authMiddleware, func(c *gin.Context) {
		s.handleGetCredentialEdit(c)
	})

	// 更新凭据
	r.POST("/credentials/:id", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostCredentialUpdate(c)
	})

	// 启用/禁用凭据
	r.POST("/credentials/:id/status", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostCredentialStatus(c)
	})

	// 删除凭据
	r.POST("/credentials/:id/delete", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostCredentialDelete(c)
	})

	// 设为默认凭据
	r.POST("/credentials/:id/set-default", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostCredentialSetDefault(c)
	})

	// 切换当前凭据
	r.POST("/credentials/select", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostCredentialSelect(c)
	})

	// ===== 账号路由 =====

	// 账号列表
	r.GET("/accounts", authMiddleware, func(c *gin.Context) {
		s.handleGetAccounts(c)
	})

	// 账号登录向导（单页登录流程）
	r.GET("/accounts/login", authMiddleware, func(c *gin.Context) {
		s.handleGetAccountLogin(c)
	})

	// GET /accounts/login/start 不应返回 CSRF 403，重定向回登录页
	r.GET("/accounts/login/start", authMiddleware, func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/accounts/login")
	})

	// 传统 form POST fallback（保留兼容，但主路径使用异步 API）
	r.POST("/accounts/login/start", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountLoginStart(c)
	})

	// 验证码输入页（兼容旧路由，重定向回登录页）
	r.GET("/accounts/login/code", authMiddleware, func(c *gin.Context) {
		s.handleGetAccountLoginCode(c)
	})

	// 提交验证码（传统 fallback）
	r.POST("/accounts/login/code", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountLoginCode(c)
	})

	// 2FA 密码输入页（兼容旧路由）
	r.GET("/accounts/login/password", authMiddleware, func(c *gin.Context) {
		s.handleGetAccountLoginPassword(c)
	})

	// 提交 2FA 密码
	r.POST("/accounts/login/password", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountLoginPassword(c)
	})

	// 账号详情
	r.GET("/accounts/:id", authMiddleware, func(c *gin.Context) {
		s.handleGetAccountDetail(c)
	})

	// 远端 Logout
	r.POST("/accounts/:id/logout", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountLogout(c)
	})

	// 本地删除 Session
	r.POST("/accounts/:id/delete-session", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountDeleteSession(c)
	})

	// 同步账号资料
	r.POST("/accounts/:id/sync", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountSync(c)
	})

	// 检测 Session 状态
	r.POST("/accounts/:id/check-session", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handlePostAccountCheckSession(c)
	})

	// ===== 异步登录 API =====

	// 发送验证码
	r.POST("/api/accounts/login/start", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handleAPILoginStart(c)
	})

	// 提交验证码
	r.POST("/api/accounts/login/code", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handleAPILoginCode(c)
	})

	// 提交 2FA 密码
	r.POST("/api/accounts/login/password", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handleAPILoginPassword(c)
	})

	// 取消登录流程
	r.POST("/api/accounts/login/cancel", authMiddleware, csrfMiddleware, func(c *gin.Context) {
		s.handleAPILoginCancel(c)
	})

	// ===== 占位路由 =====

	placeholderRoutes := []string{"/audit", "/security"}
	for _, route := range placeholderRoutes {
		r.GET(route, authMiddleware, func(c *gin.Context) {
			data := s.newAuthViewData(c, "placeholder")
			c.HTML(http.StatusOK, "placeholder.html", data)
		})
	}

	// 404 处理
	r.NoRoute(func(c *gin.Context) {
		data := NewViewData(s.cfg, "404")
		c.Status(http.StatusNotFound)
		c.HTML(http.StatusNotFound, "404.html", data.ToMap())
	})
}

// newAuthViewData 创建已认证页面的 ViewData。
func (s *Server) newAuthViewData(c *gin.Context, activeNav string) map[string]any {
	data := NewViewData(s.cfg, activeNav)
	data.IsInitialized = true
	data.IsAuthenticated = true
	data.CurrentAdminUsername = auth.GetUsername(c)

	// 生成 CSRF token
	token := s.setCSRFToken(c)
	data.CSRFToken = token

	// 转换为 map
	result := data.ToMap()

	// 获取账号切换器数据
	accountList, currentAccount := s.getAccountSwitcherData(c)
	result["AccountList"] = accountList
	if currentAccount != nil {
		result["CurrentAccountID"] = currentAccount.ID
		result["CurrentAccountName"] = currentAccount.DisplayName
		result["CurrentAccountUsername"] = currentAccount.Username
	}

	return result
}

// AccountSwitcherItem 是账号切换器的项目。
type AccountSwitcherItem struct {
	ID          uint
	DisplayName string
	Username    string
	PhoneMasked string
	IsCurrent   bool
}

// getAccountSwitcherData 获取账号切换器数据。
func (s *Server) getAccountSwitcherData(c *gin.Context) ([]AccountSwitcherItem, *AccountSwitcherItem) {
	// 获取当前选中的账号 ID
	selectedID := auth.GetSelectedAccountID(c)

	// 查询所有活跃账号
	var accounts []model.TelegramAccount
	s.db.Where("status IN ?", []string{"active", "logged_out"}).
		Order("id ASC").Find(&accounts)

	if len(accounts) == 0 {
		return nil, nil
	}

	items := make([]AccountSwitcherItem, 0, len(accounts))
	var current *AccountSwitcherItem

	for _, acc := range accounts {
		item := AccountSwitcherItem{
			ID:          acc.ID,
			DisplayName: acc.DisplayName,
			Username:    acc.Username,
			PhoneMasked: acc.PhoneFingerprint,
			IsCurrent:   acc.ID == selectedID,
		}
		items = append(items, item)

		if acc.ID == selectedID {
			current = &item
		}
	}

	// 如果没有选中但有账号，默认选第一个
	if current == nil && len(items) > 0 {
		items[0].IsCurrent = true
		current = &items[0]
	}

	return items, current
}
