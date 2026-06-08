package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gotd/td/telegram/dcs"
	"github.com/user/atria/internal/account"
	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/mtproto"
	"github.com/user/atria/internal/network"
	"github.com/user/atria/internal/security"

	"github.com/gin-gonic/gin"
)

// handleGetAccounts 处理 GET /accounts - 账号列表。
func (s *Server) handleGetAccounts(c *gin.Context) {
	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)

	accounts, err := accountSvc.ListAccounts(c.Request.Context())
	if err != nil {
		slog.Error("查询账号列表失败", "error", err)
		RenderError(c, http.StatusInternalServerError, "服务器错误", "查询账号失败")
		return
	}

	// 获取系统 API Key
	credSvc := credential.NewService(s.db, s.key)
	systemKey, _ := credSvc.GetSystemAPIKey()
	hasSystemKey := systemKey != nil

	data := s.newAccountViewData(c, "accounts")
	data["Accounts"] = accounts
	data["HasDefaultCredential"] = hasSystemKey
	if hasSystemKey {
		data["DefaultCredentialName"] = systemKey.DisplayName
	}
	c.HTML(http.StatusOK, "accounts.html", data)
}

// handleGetAccountLogin 处理 GET /accounts/login - 登录向导页面。
func (s *Server) handleGetAccountLogin(c *gin.Context) {
	credSvc := credential.NewService(s.db, s.key)

	// 获取系统 API Key
	systemKey, _ := credSvc.GetSystemAPIKey()
	hasSystemKey := systemKey != nil

	data := s.newAccountViewData(c, "accounts")
	data["HasDefaultCredential"] = hasSystemKey

	if hasSystemKey {
		data["DefaultCredentialName"] = systemKey.DisplayName
		data["DefaultCredentialID"] = systemKey.ID
	}

	c.HTML(http.StatusOK, "account_login.html", data)
}

// handlePostAccountLoginStart 处理 POST /accounts/login/start - 开始登录流程。
func (s *Server) handlePostAccountLoginStart(c *gin.Context) {
	// 使用系统 API Key
	credSvc := credential.NewService(s.db, s.key)
	systemKey, _ := credSvc.GetSystemAPIKey()
	if systemKey == nil {
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = "请先在系统设置中配置 Telegram API Key"
		data["HasDefaultCredential"] = false
		c.HTML(http.StatusOK, "account_login.html", data)
		return
	}

	credID := systemKey.ID

	phone := c.PostForm("phone")
	if err := account.ValidatePhone(phone); err != nil {
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = err.Error()
		data["HasDefaultCredential"] = true
		data["DefaultCredentialName"] = systemKey.DisplayName
		data["DefaultCredentialID"] = credID
		c.HTML(http.StatusOK, "account_login.html", data)
		return
	}

	apiHash, err := security.DecryptAPIHash(s.key, systemKey.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		RenderError(c, http.StatusInternalServerError, "服务器错误", "解密凭据失败")
		return
	}

	flowID := fmt.Sprintf("flow_%d_%s", credID, crypto.Fingerprint(phone)[:8])
	phoneEncrypted, phoneFingerprint, _ := security.EncryptPhone(s.key, phone)

	flow := mtproto.NewLoginFlow(flowID, credID, int(systemKey.APIID), phoneEncrypted, phoneFingerprint)
	if err := s.flowStore.Create(c.Request.Context(), flow); err != nil {
		RenderError(c, http.StatusInternalServerError, "操作失败", "创建登录流程失败")
		return
	}

	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	step, err := client.StartLogin(c.Request.Context(), mtproto.StartLoginRequest{
		APICredentialID: credID,
		APIID:           int(systemKey.APIID),
		APIHash:         apiHash,
		Phone:           phone,
		FlowID:          flowID,
	})

	if err != nil {
		s.flowStore.Delete(c.Request.Context(), flowID)

		errKind := mtproto.ClassifyError(err)
		errMsg := getErrorMessage(errKind, err)

		data := s.newAccountViewData(c, "accounts")
		data["Error"] = errMsg
		data["HasDefaultCredential"] = true
		data["DefaultCredentialName"] = systemKey.DisplayName
		data["DefaultCredentialID"] = credID
		c.HTML(http.StatusOK, "account_login.html", data)
		return
	}

	// 防御：step 为 nil 或 FlowID 为空时不得 panic
	if step == nil || step.FlowID == "" {
		s.flowStore.Delete(c.Request.Context(), flowID)
		slog.Error("登录流程启动失败：返回了空的步骤结果")

		data := s.newAccountViewData(c, "accounts")
		data["Error"] = "登录流程启动失败，请检查 Telegram API Key 和 API 网络代理配置。"
		data["HasDefaultCredential"] = true
		data["DefaultCredentialName"] = systemKey.DisplayName
		data["DefaultCredentialID"] = credID
		c.HTML(http.StatusOK, "account_login.html", data)
		return
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
		Action:       "account.login_code_sent",
		ResourceType: "login_flow",
		ResourceID:   flowID,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "验证码已发送",
		Metadata: map[string]any{
			"api_credential_id": credID,
			"flow_id":           flowID,
		},
	})

	c.Redirect(http.StatusFound, "/accounts/login/code?flow_id="+step.FlowID)
}

// handleGetAccountLoginCode 处理 GET /accounts/login/code - 验证码输入页。
// 兼容旧路由：flow_id 无效时重定向回登录页。
func (s *Server) handleGetAccountLoginCode(c *gin.Context) {
	flowID := c.Query("flow_id")
	if flowID == "" {
		c.Redirect(http.StatusFound, "/accounts/login")
		return
	}

	flow, err := s.flowStore.Get(c.Request.Context(), flowID)
	if err != nil {
		c.Redirect(http.StatusFound, "/accounts/login")
		return
	}

	if flow.State != mtproto.LoginStateCodeSent {
		c.Redirect(http.StatusFound, "/accounts/login")
		return
	}

	data := s.newAccountViewData(c, "accounts")
	data["FlowID"] = flowID
	data["PhoneHint"] = flow.PhoneFingerprint
	c.HTML(http.StatusOK, "account_code.html", data)
}

// handlePostAccountLoginCode 处理 POST /accounts/login/code - 提交验证码。
func (s *Server) handlePostAccountLoginCode(c *gin.Context) {
	flowID := c.PostForm("flow_id")
	code := c.PostForm("code")

	if flowID == "" {
		RenderError(c, http.StatusBadRequest, "请求无效", "缺少流程 ID")
		return
	}

	if code == "" {
		data := s.newAccountViewData(c, "accounts")
		data["FlowID"] = flowID
		data["Error"] = "验证码不能为空"
		c.HTML(http.StatusOK, "account_code.html", data)
		return
	}

	flow, err := s.flowStore.Get(c.Request.Context(), flowID)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "流程无效", "登录流程不存在或已过期")
		return
	}

	credSvc := credential.NewService(s.db, s.key)
	cred, err := credSvc.GetByID(flow.APICredentialID)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "操作失败", "API 凭据不存在")
		return
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		RenderError(c, http.StatusInternalServerError, "服务器错误", "解密凭据失败")
		return
	}

	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	step, err := client.SubmitCode(c.Request.Context(), mtproto.SubmitCodeRequest{
		FlowID:  flowID,
		Code:    code,
		APIID:   flow.APIID,
		APIHash: apiHash,
	})

	if err != nil {
		errKind := mtproto.ClassifyError(err)
		errMsg := getErrorMessage(errKind, err)

		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
			Action:       "account.login_code_failed",
			ResourceType: "login_flow",
			ResourceID:   flowID,
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "验证码提交失败",
			Metadata: map[string]any{
				"error_kind":        string(errKind),
				"api_credential_id": flow.APICredentialID,
			},
		})

		data := s.newAccountViewData(c, "accounts")
		data["FlowID"] = flowID
		data["Error"] = errMsg
		c.HTML(http.StatusOK, "account_code.html", data)
		return
	}

	if step.State == mtproto.LoginStateWaitingPassword {
		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
			Action:       "account.login_password_required",
			ResourceType: "login_flow",
			ResourceID:   flowID,
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "需要两步验证密码",
			Metadata: map[string]any{
				"api_credential_id": flow.APICredentialID,
			},
		})

		c.Redirect(http.StatusFound, "/accounts/login/password?flow_id="+flowID)
		return
	}

	if step.State == mtproto.LoginStateAuthorized {
		s.completeLogin(c, flow, step)
		return
	}

	data := s.newAccountViewData(c, "accounts")
	data["FlowID"] = flowID
	data["Error"] = "登录流程状态异常"
	c.HTML(http.StatusOK, "account_code.html", data)
}

// handleGetAccountLoginPassword 处理 GET /accounts/login/password - 2FA 密码输入页。
func (s *Server) handleGetAccountLoginPassword(c *gin.Context) {
	flowID := c.Query("flow_id")
	if flowID == "" {
		RenderError(c, http.StatusBadRequest, "请求无效", "缺少流程 ID")
		return
	}

	flow, err := s.flowStore.Get(c.Request.Context(), flowID)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "流程无效", "登录流程不存在或已过期")
		return
	}

	if flow.State != mtproto.LoginStateWaitingPassword {
		RenderError(c, http.StatusBadRequest, "流程状态错误", "当前流程状态不正确")
		return
	}

	data := s.newAccountViewData(c, "accounts")
	data["FlowID"] = flowID
	c.HTML(http.StatusOK, "account_password.html", data)
}

// handlePostAccountLoginPassword 处理 POST /accounts/login/password - 提交 2FA 密码。
func (s *Server) handlePostAccountLoginPassword(c *gin.Context) {
	flowID := c.PostForm("flow_id")
	password := c.PostForm("password")

	if flowID == "" {
		RenderError(c, http.StatusBadRequest, "请求无效", "缺少流程 ID")
		return
	}

	if password == "" {
		data := s.newAccountViewData(c, "accounts")
		data["FlowID"] = flowID
		data["Error"] = "密码不能为空"
		c.HTML(http.StatusOK, "account_password.html", data)
		return
	}

	flow, err := s.flowStore.Get(c.Request.Context(), flowID)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "流程无效", "登录流程不存在或已过期")
		return
	}

	credSvc := credential.NewService(s.db, s.key)
	cred, err := credSvc.GetByID(flow.APICredentialID)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "操作失败", "API 凭据不存在")
		return
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		RenderError(c, http.StatusInternalServerError, "服务器错误", "解密凭据失败")
		return
	}

	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	step, err := client.SubmitPassword(c.Request.Context(), mtproto.SubmitPasswordRequest{
		FlowID:   flowID,
		Password: password,
		APIID:    flow.APIID,
		APIHash:  apiHash,
	})

	if err != nil {
		errKind := mtproto.ClassifyError(err)
		errMsg := getErrorMessage(errKind, err)

		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
			Action:       "account.login_password_failed",
			ResourceType: "login_flow",
			ResourceID:   flowID,
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "两步验证密码提交失败",
			Metadata: map[string]any{
				"error_kind":        string(errKind),
				"api_credential_id": flow.APICredentialID,
			},
		})

		data := s.newAccountViewData(c, "accounts")
		data["FlowID"] = flowID
		data["Error"] = errMsg
		c.HTML(http.StatusOK, "account_password.html", data)
		return
	}

	if step.State == mtproto.LoginStateAuthorized {
		s.completeLogin(c, flow, step)
		return
	}

	data := s.newAccountViewData(c, "accounts")
	data["FlowID"] = flowID
	data["Error"] = "登录流程状态异常"
	c.HTML(http.StatusOK, "account_password.html", data)
}

// completeLogin 完成登录流程。
func (s *Server) completeLogin(c *gin.Context, flow *mtproto.LoginFlow, step *mtproto.LoginStep) {
	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)
	actorID := auth.GetAdminID(c)

	if step.Account == nil {
		slog.Error("登录成功但未获取到账号资料")
		RenderError(c, http.StatusInternalServerError, "服务器错误", "登录成功但未获取到账号资料")
		return
	}

	acc, err := accountSvc.CompleteLogin(c.Request.Context(), account.CompleteLoginInput{
		APICredentialID: flow.APICredentialID,
		Profile:         step.Account,
		SessionData:     step.SessionData,
		ActorID:         actorID,
		IP:              c.ClientIP(),
		UserAgent:       c.GetHeader("User-Agent"),
	})

	if err != nil {
		slog.Error("创建/更新账号失败", "error", err)
		RenderError(c, http.StatusInternalServerError, "服务器错误", "创建账号失败")
		return
	}

	// 清理临时 Session
	tmpStorage := mtproto.NewGotdSessionStorage(s.cfg.SessionDir+"/tmp", s.key, "flow_"+flow.ID)
	tmpStorage.DeleteSession()

	// 删除 Flow
	s.flowStore.Delete(c.Request.Context(), flow.ID)

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", actorID),
		Action:       "account.login_authorized",
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", acc.ID),
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "账号登录成功",
		Metadata: map[string]any{
			"user_id":           acc.UserID,
			"api_credential_id": flow.APICredentialID,
		},
	})

	c.Redirect(http.StatusFound, fmt.Sprintf("/accounts/%d", acc.ID))
}

// handlePostAccountSync 处理 POST /accounts/:id/sync - 同步账号资料。
func (s *Server) handlePostAccountSync(c *gin.Context) {
	id, err := parseAccountID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "账号 ID 不合法")
		return
	}

	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)

	actorID := auth.GetAdminID(c)
	result, err := accountSvc.SyncProfile(c.Request.Context(), account.SyncProfileInput{
		AccountID: id,
		ActorID:   actorID,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})

	if err != nil {
		// 显示友好错误
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = err.Error()

		// 重新加载账号信息
		acc, _ := accountSvc.GetAccount(c.Request.Context(), id)
		data["Account"] = acc

		c.HTML(http.StatusOK, "account_detail.html", data)
		return
	}

	_ = result
	c.Redirect(http.StatusFound, fmt.Sprintf("/accounts/%d", id))
}

// handlePostAccountCheckSession 处理 POST /accounts/:id/check-session - 检测 Session 状态。
func (s *Server) handlePostAccountCheckSession(c *gin.Context) {
	id, err := parseAccountID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "账号 ID 不合法")
		return
	}

	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)

	actorID := auth.GetAdminID(c)
	result, err := accountSvc.CheckSession(c.Request.Context(), account.CheckSessionInput{
		AccountID: id,
		ActorID:   actorID,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})

	if err != nil {
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = err.Error()

		acc, _ := accountSvc.GetAccount(c.Request.Context(), id)
		data["Account"] = acc

		c.HTML(http.StatusOK, "account_detail.html", data)
		return
	}

	_ = result
	c.Redirect(http.StatusFound, fmt.Sprintf("/accounts/%d", id))
}

// handleGetAccountDetail 处理 GET /accounts/:id - 账号详情页。
func (s *Server) handleGetAccountDetail(c *gin.Context) {
	id, err := parseAccountID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "账号 ID 不合法")
		return
	}

	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)

	acc, err := accountSvc.GetAccount(c.Request.Context(), id)
	if err != nil {
		RenderError(c, http.StatusNotFound, "未找到", "账号不存在")
		return
	}

	data := s.newAccountViewData(c, "accounts")
	data["Account"] = acc
	c.HTML(http.StatusOK, "account_detail.html", data)
}

// handlePostAccountLogout 处理 POST /accounts/:id/logout - 远端 Logout。
func (s *Server) handlePostAccountLogout(c *gin.Context) {
	id, err := parseAccountID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "账号 ID 不合法")
		return
	}

	// 服务端确认字段校验
	confirm := c.PostForm("confirm")
	if confirm != "remote_logout" {
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = "缺少确认字段，请重试"
		acc, _ := s.getAccountService().GetAccount(c.Request.Context(), id)
		data["Account"] = acc
		c.HTML(http.StatusOK, "account_detail.html", data)
		return
	}

	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)

	actorID := auth.GetAdminID(c)
	if err := accountSvc.RemoteLogout(c.Request.Context(), account.RemoteLogoutInput{
		AccountID: id,
		ActorID:   actorID,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}); err != nil {
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = err.Error()
		acc, _ := accountSvc.GetAccount(c.Request.Context(), id)
		data["Account"] = acc
		c.HTML(http.StatusOK, "account_detail.html", data)
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/accounts/%d", id))
}

// handlePostAccountDeleteSession 处理 POST /accounts/:id/delete-session - 本地删除 Session。
func (s *Server) handlePostAccountDeleteSession(c *gin.Context) {
	id, err := parseAccountID(c)
	if err != nil {
		RenderError(c, http.StatusBadRequest, "请求无效", "账号 ID 不合法")
		return
	}

	// 服务端确认字段校验
	confirm := c.PostForm("confirm")
	if confirm != "delete_local_session" {
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = "缺少确认字段，请重试"
		acc, _ := s.getAccountService().GetAccount(c.Request.Context(), id)
		data["Account"] = acc
		c.HTML(http.StatusOK, "account_detail.html", data)
		return
	}

	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)

	actorID := auth.GetAdminID(c)
	if err := accountSvc.DeleteLocalSession(c.Request.Context(), account.DeleteLocalSessionInput{
		AccountID: id,
		ActorID:   actorID,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}); err != nil {
		data := s.newAccountViewData(c, "accounts")
		data["Error"] = err.Error()
		acc, _ := accountSvc.GetAccount(c.Request.Context(), id)
		data["Account"] = acc
		c.HTML(http.StatusOK, "account_detail.html", data)
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/accounts/%d", id))
}

// getAccountService 创建账号服务实例。
func (s *Server) getAccountService() *account.Service {
	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	return account.NewService(s.db, s.key, sessionStore, client)
}

// ===== 异步登录 API =====

// handleAPILoginStart 处理 POST /api/accounts/login/start - 异步发送验证码。
func (s *Server) handleAPILoginStart(c *gin.Context) {
	credSvc := credential.NewService(s.db, s.key)
	systemKey, _ := credSvc.GetSystemAPIKey()
	if systemKey == nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "no_api_key",
			"message": "请先在系统设置中配置 Telegram API Key。",
		})
		return
	}

	credID := systemKey.ID
	phone := c.PostForm("phone")
	if err := account.ValidatePhone(phone); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "invalid_phone",
			"message": "请输入完整的国际手机号，例如 +8613800000000。",
		})
		return
	}

	apiHash, err := security.DecryptAPIHash(s.key, systemKey.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "internal_error",
			"message": "解密凭据失败，请检查系统设置。",
		})
		return
	}

	flowID := fmt.Sprintf("flow_%d_%s", credID, crypto.Fingerprint(phone)[:8])
	phoneEncrypted, phoneFingerprint, _ := security.EncryptPhone(s.key, phone)

	flow := mtproto.NewLoginFlow(flowID, credID, int(systemKey.APIID), phoneEncrypted, phoneFingerprint)
	if err := s.flowStore.Create(c.Request.Context(), flow); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "flow_create_failed",
			"message": "登录流程启动失败，请检查 Telegram API Key 和 API 网络代理配置。",
		})
		return
	}

	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, proxyErr := s.proxyDialerFromSettings()
	if proxyErr != nil {
		slog.Error("代理配置错误", "error", proxyErr)
		s.flowStore.Delete(c.Request.Context(), flowID)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "proxy_config_invalid",
			"message": proxyErr.Error(),
		})
		return
	}
	if dialer != nil {
		client.SetDialer(dialer)
	}
	step, err := client.StartLogin(c.Request.Context(), mtproto.StartLoginRequest{
		APICredentialID: credID,
		APIID:           int(systemKey.APIID),
		APIHash:         apiHash,
		Phone:           phone,
		FlowID:          flowID,
	})

	if err != nil {
		s.flowStore.Delete(c.Request.Context(), flowID)
		errKind := mtproto.ClassifyError(err)
		errMsg := getErrorMessage(errKind, err)

		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    string(errKind),
			"message": errMsg,
		})
		return
	}

	if step == nil || step.FlowID == "" {
		s.flowStore.Delete(c.Request.Context(), flowID)
		slog.Error("登录流程启动失败：返回了空的步骤结果")
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "internal_error",
			"message": "登录流程启动失败，请检查 Telegram API Key 和 API 网络代理配置。",
		})
		return
	}

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
		Action:       "account.login_code_sent",
		ResourceType: "login_flow",
		ResourceID:   flowID,
		RiskLevel:    "low",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "验证码已发送",
		Metadata: map[string]any{
			"api_credential_id": credID,
			"flow_id":           flowID,
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"next":    "code",
		"flow_id": step.FlowID,
		"message": "验证码已发送，请填写 Telegram 发送的验证码。",
	})
}

// handleAPILoginCode 处理 POST /api/accounts/login/code - 异步提交验证码。
func (s *Server) handleAPILoginCode(c *gin.Context) {
	flowID := c.PostForm("flow_id")
	code := c.PostForm("code")

	if flowID == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "missing_flow_id",
			"message": "缺少流程 ID，请重新开始登录。",
		})
		return
	}

	if code == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "missing_code",
			"message": "验证码不能为空。",
		})
		return
	}

	flow, err := s.flowStore.Get(c.Request.Context(), flowID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "flow_expired",
			"message": "登录流程已过期，请重新开始。",
		})
		return
	}

	credSvc := credential.NewService(s.db, s.key)
	cred, err := credSvc.GetByID(flow.APICredentialID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "internal_error",
			"message": "API 凭据不存在，请重新开始。",
		})
		return
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "internal_error",
			"message": "解密凭据失败。",
		})
		return
	}

	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, proxyErr := s.proxyDialerFromSettings()
	if proxyErr != nil {
		slog.Error("代理配置错误", "error", proxyErr)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "proxy_config_invalid",
			"message": proxyErr.Error(),
		})
		return
	}
	if dialer != nil {
		client.SetDialer(dialer)
	}

	slog.Info("SubmitCode 请求",
		"flow_id", flowID,
		"has_code", code != "",
		"has_phone_code_hash", flow.PhoneCodeHashEncrypted != "",
		"api_credential_id", flow.APICredentialID,
		"has_dialer", dialer != nil,
	)

	step, err := client.SubmitCode(c.Request.Context(), mtproto.SubmitCodeRequest{
		FlowID:  flowID,
		Code:    code,
		APIID:   flow.APIID,
		APIHash: apiHash,
	})

	if err != nil {
		errKind := mtproto.ClassifyError(err)
		errMsg := getErrorMessage(errKind, err)

		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
			Action:       "account.login_code_failed",
			ResourceType: "login_flow",
			ResourceID:   flowID,
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "验证码提交失败",
			Metadata: map[string]any{
				"error_kind":        string(errKind),
				"api_credential_id": flow.APICredentialID,
			},
		})

		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    string(errKind),
			"message": errMsg,
		})
		return
	}

	if step.State == mtproto.LoginStateWaitingPassword {
		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
			Action:       "account.login_password_required",
			ResourceType: "login_flow",
			ResourceID:   flowID,
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "需要两步验证密码",
			Metadata: map[string]any{
				"api_credential_id": flow.APICredentialID,
			},
		})

		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"next":    "password",
			"flow_id": flowID,
			"message": "该账号已开启两步验证，请输入 2FA 密码。",
		})
		return
	}

	if step.State == mtproto.LoginStateAuthorized {
		acc, loginErr := s.completeLoginJSON(c, flow, step)
		if loginErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"ok":      false,
				"code":    "complete_failed",
				"message": loginErr.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":       true,
			"next":     "done",
			"redirect": fmt.Sprintf("/accounts/%d", acc.ID),
			"message":  "登录成功",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      false,
		"code":    "internal_error",
		"message": "登录流程状态异常，请重新开始。",
	})
}

// handleAPILoginPassword 处理 POST /api/accounts/login/password - 异步提交 2FA 密码。
func (s *Server) handleAPILoginPassword(c *gin.Context) {
	flowID := c.PostForm("flow_id")
	password := c.PostForm("password")

	if flowID == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "missing_flow_id",
			"message": "缺少流程 ID，请重新开始登录。",
		})
		return
	}

	if password == "" {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "missing_password",
			"message": "密码不能为空。",
		})
		return
	}

	flow, err := s.flowStore.Get(c.Request.Context(), flowID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "flow_expired",
			"message": "登录流程已过期，请重新开始。",
		})
		return
	}

	credSvc := credential.NewService(s.db, s.key)
	cred, err := credSvc.GetByID(flow.APICredentialID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "internal_error",
			"message": "API 凭据不存在，请重新开始。",
		})
		return
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "internal_error",
			"message": "解密凭据失败。",
		})
		return
	}

	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, proxyErr := s.proxyDialerFromSettings()
	if proxyErr != nil {
		slog.Error("代理配置错误", "error", proxyErr)
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    "proxy_config_invalid",
			"message": proxyErr.Error(),
		})
		return
	}
	if dialer != nil {
		client.SetDialer(dialer)
	}
	step, err := client.SubmitPassword(c.Request.Context(), mtproto.SubmitPasswordRequest{
		FlowID:   flowID,
		Password: password,
		APIID:    flow.APIID,
		APIHash:  apiHash,
	})

	if err != nil {
		errKind := mtproto.ClassifyError(err)
		errMsg := getErrorMessage(errKind, err)

		audit.Log(c.Request.Context(), s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", auth.GetAdminID(c)),
			Action:       "account.login_password_failed",
			ResourceType: "login_flow",
			ResourceID:   flowID,
			RiskLevel:    "medium",
			IP:           c.ClientIP(),
			UserAgent:    c.GetHeader("User-Agent"),
			Message:      "两步验证密码提交失败",
			Metadata: map[string]any{
				"error_kind":        string(errKind),
				"api_credential_id": flow.APICredentialID,
			},
		})

		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"code":    string(errKind),
			"message": errMsg,
		})
		return
	}

	if step.State == mtproto.LoginStateAuthorized {
		acc, loginErr := s.completeLoginJSON(c, flow, step)
		if loginErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"ok":      false,
				"code":    "complete_failed",
				"message": loginErr.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":       true,
			"next":     "done",
			"redirect": fmt.Sprintf("/accounts/%d", acc.ID),
			"message":  "登录成功",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      false,
		"code":    "internal_error",
		"message": "登录流程状态异常，请重新开始。",
	})
}

// handleAPILoginCancel 处理 POST /api/accounts/login/cancel - 取消登录流程。
func (s *Server) handleAPILoginCancel(c *gin.Context) {
	flowID := c.PostForm("flow_id")
	if flowID != "" {
		s.flowStore.Delete(c.Request.Context(), flowID)
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "登录流程已取消。",
	})
}

// completeLoginJSON 完成登录流程（JSON 版本），返回账号或错误。
func (s *Server) completeLoginJSON(c *gin.Context, flow *mtproto.LoginFlow, step *mtproto.LoginStep) (*model.TelegramAccount, error) {
	sessionStore := mtproto.NewFileSessionStore(s.cfg.SessionDir, s.key)
	client := mtproto.NewGotdClient(s.cfg.SessionDir, s.key, s.flowStore, slog.Default())
	dialer, _ := s.proxyDialerFromSettings()
	if dialer != nil {
		client.SetDialer(dialer)
	}
	accountSvc := account.NewService(s.db, s.key, sessionStore, client)
	actorID := auth.GetAdminID(c)

	if step.Account == nil {
		slog.Error("登录成功但未获取到账号资料")
		return nil, fmt.Errorf("登录成功但未获取到账号资料")
	}

	acc, err := accountSvc.CompleteLogin(c.Request.Context(), account.CompleteLoginInput{
		APICredentialID: flow.APICredentialID,
		Profile:         step.Account,
		SessionData:     step.SessionData,
		ActorID:         actorID,
		IP:              c.ClientIP(),
		UserAgent:       c.GetHeader("User-Agent"),
	})
	if err != nil {
		slog.Error("创建/更新账号失败", "error", err)
		return nil, fmt.Errorf("创建账号失败")
	}

	// 清理临时 Session
	tmpStorage := mtproto.NewGotdSessionStorage(s.cfg.SessionDir+"/tmp", s.key, "flow_"+flow.ID)
	tmpStorage.DeleteSession()

	// 删除 Flow
	s.flowStore.Delete(c.Request.Context(), flow.ID)

	audit.Log(c.Request.Context(), s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", actorID),
		Action:       "account.login_authorized",
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", acc.ID),
		RiskLevel:    "medium",
		IP:           c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		Message:      "账号登录成功",
		Metadata: map[string]any{
			"user_id":           acc.UserID,
			"api_credential_id": flow.APICredentialID,
		},
	})

	return acc, nil
}

// getErrorMessage 获取用户友好的错误消息。

// proxyDialerFromSettings 从数据库读取代理配置，返回 gotd 兼容的 DialFunc。
// 如果代理未启用或类型为 none，返回 nil（直连）。
// 如果代理配置不完整或解密失败，返回 error。
func (s *Server) proxyDialerFromSettings() (dcs.DialFunc, error) {
	var settings []model.SystemSetting
	s.db.Where("key IN ?", []string{"proxy_enabled", "proxy_type", "proxy_host", "proxy_port", "proxy_username", "proxy_timeout"}).Find(&settings)

	settingMap := make(map[string]string, len(settings))
	for _, st := range settings {
		settingMap[st.Key] = st.Value
	}

	// 检查代理是否启用
	if settingMap["proxy_enabled"] != "true" && settingMap["proxy_type"] == "none" {
		return nil, nil
	}

	proxyType := settingMap["proxy_type"]
	if proxyType == "none" || proxyType == "" {
		return nil, nil
	}

	host := settingMap["proxy_host"]
	portStr := settingMap["proxy_port"]
	if host == "" || portStr == "" {
		slog.Warn("代理配置不完整：缺少主机或端口", "host", host, "port", portStr)
		return nil, fmt.Errorf("代理配置不完整，请检查代理类型、主机和端口")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		slog.Warn("代理端口无效", "port", portStr)
		return nil, fmt.Errorf("代理端口无效: %s", portStr)
	}

	timeout := 30 * time.Second
	if t := settingMap["proxy_timeout"]; t != "" {
		if secs, err := strconv.Atoi(t); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}

	username := settingMap["proxy_username"]

	// 读取代理密码（加密存储）
	// proxy_password 记录缺失时视为空密码（合法）
	// proxy_password 存在但解密失败时返回错误，不得静默降级
	password := ""
	var pwdSetting model.SystemSetting
	if err := s.db.Where("key = ?", "proxy_password").First(&pwdSetting).Error; err == nil && pwdSetting.Value != "" {
		decrypted, err := crypto.DecryptString(s.key, pwdSetting.Value, []byte("atria:proxy:v1"))
		if err != nil {
			slog.Error("解密代理密码失败，请检查代理配置", "error", err)
			return nil, fmt.Errorf("代理密码配置错误，请重新配置代理")
		}
		password = decrypted
	}

	slog.Info("创建代理拨号器",
		"type", proxyType,
		"host", host,
		"port", port,
		"has_username", username != "",
		"has_password", password != "",
	)

	config := network.ProxyConfig{
		Type:     network.ProxyType(proxyType),
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Timeout:  timeout,
	}

	dialer := network.NewDialer(config)
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}, nil
}
func getErrorMessage(kind mtproto.ErrorKind, err error) string {
	switch kind {
	case mtproto.ErrFloodWait:
		if floodErr, ok := err.(*mtproto.FloodWaitError); ok {
			return fmt.Sprintf("操作过于频繁，请等待 %s 后重试", floodErr.Wait)
		}
		return "操作过于频繁，请稍后重试"
	case mtproto.ErrInvalidPhone:
		return "请输入完整的国际手机号，例如 +8613800000000。"
	case mtproto.ErrLoginCodeInvalid:
		return "验证码错误，请检查后重新输入。"
	case mtproto.ErrLoginCodeExpired:
		return "验证码已过期，请重新开始登录流程。"
	case mtproto.ErrLoginPasswordInvalid:
		return "2FA 密码错误，请重新输入。"
	case mtproto.ErrUnauthorized:
		return "账号已被封禁或限制。"
	case mtproto.ErrCredentialDisabled:
		return "Telegram API Key 不可用，请检查 API ID / API Hash。"
	case mtproto.ErrSessionInvalid:
		return "Session 已失效，请重新登录该账号。"
	case mtproto.ErrProxyConnectFailed:
		return "无法连接到代理服务器，请检查代理地址和端口。"
	case mtproto.ErrProxyAuthFailed:
		return "代理认证失败，请检查用户名和密码。"
	case mtproto.ErrTelegramTimeout:
		return "连接 Telegram 超时，请检查 API 网络代理配置。"
	case mtproto.ErrNetworkError:
		return "网络异常，请检查网络连接或代理配置。"
	default:
		return "操作失败，请稍后重试或检查日志。"
	}
}

// newAccountViewData 创建账号页面的 ViewData。
func (s *Server) newAccountViewData(c *gin.Context, activeNav string) map[string]any {
	data := NewViewData(s.cfg, activeNav)
	data.IsInitialized = true
	data.IsAuthenticated = true
	data.CurrentAdminUsername = auth.GetUsername(c)

	token := s.setCSRFToken(c)
	data.CSRFToken = token

	credID := auth.GetCredentialID(c)
	if credID > 0 {
		credSvc := credential.NewService(s.db, s.key)
		cred, err := credSvc.GetByID(credID)
		if err == nil {
			data.CurrentCredentialID = credID
			data.CurrentCredentialName = cred.DisplayName
		}
	}

	return data.ToMap()
}

// parseAccountID 从 URL 参数解析账号 ID。
func parseAccountID(c *gin.Context) (uint, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("ID 不合法")
	}
	return uint(id), nil
}

// credentialInfo 是凭据信息的临时结构。
type credentialInfo = model.APICredential
