package mtproto

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/security"
)

// GotdClient 是基于 gotd/td 的 MTProto 客户端实现。
type GotdClient struct {
	sessionDir string
	key        []byte
	flowStore  FlowStore
	logger     *slog.Logger
	dialFunc   dcs.DialFunc // 自定义拨号函数，用于代理；nil 表示直连
}

// NewGotdClient 创建 GotdClient 实例。
func NewGotdClient(sessionDir string, key []byte, flowStore FlowStore, logger *slog.Logger) *GotdClient {
	return &GotdClient{
		sessionDir: sessionDir,
		key:        key,
		flowStore:  flowStore,
		logger:     logger,
	}
}

// SetDialer 设置自定义拨号器，用于通过代理连接 Telegram。
// 传入 nil 恢复直连。必须在调用 StartLogin / SubmitCode 等方法前设置。
func (c *GotdClient) SetDialer(dialFunc dcs.DialFunc) {
	c.dialFunc = dialFunc
}

// tmpSessionDir 返回临时 session 目录。
func (c *GotdClient) tmpSessionDir() string {
	return fmt.Sprintf("%s/tmp", c.sessionDir)
}

// buildOptions 构建 telegram.Options，注入代理 resolver（如有）。
func (c *GotdClient) buildOptions(opts telegram.Options) telegram.Options {
	if c.dialFunc != nil {
		opts.Resolver = dcs.Plain(dcs.PlainOptions{
			Dial: c.dialFunc,
		})
	}
	return opts
}

// createClient 创建 Telegram 客户端，使用 flow-specific 的 session 存储。
func (c *GotdClient) createClient(apiID int, apiHash string, flowID string) (*telegram.Client, *GotdSessionStorage) {
	storage := NewGotdSessionStorage(c.tmpSessionDir(), c.key, "flow_"+flowID)
	client := telegram.NewClient(apiID, apiHash, c.buildOptions(telegram.Options{
		SessionStorage: storage,
	}))
	return client, storage
}

// createClientFromSession 创建 Telegram 客户端，使用正式 session 文件。
func (c *GotdClient) createClientFromSession(apiID int, apiHash string, sessionFilePath string) (*telegram.Client, *FileBackedSessionStorage) {
	storage := NewFileBackedSessionStorage(c.key, sessionFilePath)
	client := telegram.NewClient(apiID, apiHash, c.buildOptions(telegram.Options{
		SessionStorage: storage,
	}))
	return client, storage
}

// StartLogin 发送验证码，开始登录流程。
func (c *GotdClient) StartLogin(ctx context.Context, req StartLoginRequest) (*LoginStep, error) {
	client, _ := c.createClient(req.APIID, req.APIHash, req.FlowID)

	var result *LoginStep

	err := client.Run(ctx, func(ctx context.Context) error {
		sentCode, err := client.API().AuthSendCode(ctx, &tg.AuthSendCodeRequest{
			PhoneNumber: req.Phone,
			APIID:       req.APIID,
			APIHash:     req.APIHash,
			Settings:    tg.CodeSettings{},
		})
		if err != nil {
			return c.classifyError(err)
		}

		code, ok := sentCode.(*tg.AuthSentCode)
		if !ok {
			return &MTProtoError{Kind: ErrInternalError, Message: "无法解析验证码响应"}
		}

		if code.PhoneCodeHash == "" {
			return &MTProtoError{Kind: ErrInternalError, Message: "无法获取 phone_code_hash"}
		}

		encryptedHash, _, err := security.EncryptPhone(c.key, code.PhoneCodeHash)
		if err != nil {
			return &MTProtoError{Kind: ErrInternalError, Message: "加密 phone_code_hash 失败", Err: err}
		}

		if c.flowStore != nil {
			flow, err := c.flowStore.Get(ctx, req.FlowID)
			if err == nil {
				flow.PhoneCodeHashEncrypted = encryptedHash
				flow.State = LoginStateCodeSent
				flow.UpdatedAt = time.Now()
				c.flowStore.Update(ctx, *flow)
			}
		}

		result = &LoginStep{
			State:     LoginStateCodeSent,
			Message:   "验证码已发送",
			PhoneHint: maskPhone(req.Phone),
			FlowID:    req.FlowID,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 防御：确保不会返回 nil step + nil error
	if result == nil {
		return nil, &MTProtoError{Kind: ErrNetworkError, Message: "登录流程启动失败，未获取到验证码发送结果"}
	}

	return result, nil
}

// SubmitCode 提交验证码。
func (c *GotdClient) SubmitCode(ctx context.Context, req SubmitCodeRequest) (*LoginStep, error) {
	if c.flowStore == nil {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "FlowStore 未初始化"}
	}

	flow, err := c.flowStore.Get(ctx, req.FlowID)
	if err != nil {
		return nil, &MTProtoError{Kind: ErrLoginExpired, Message: "登录流程不存在或已过期"}
	}

	if flow.State != LoginStateCodeSent {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "流程状态不正确"}
	}

	phoneCodeHash, err := security.DecryptPhone(c.key, flow.PhoneCodeHashEncrypted)
	if err != nil {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "解密 phone_code_hash 失败", Err: err}
	}

	phone, err := security.DecryptPhone(c.key, flow.PhoneEncrypted)
	if err != nil {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "解密手机号失败", Err: err}
	}

	client, storage := c.createClient(req.APIID, req.APIHash, req.FlowID)

	var result *LoginStep

	err = client.Run(ctx, func(ctx context.Context) error {
		signInResult, err := client.API().AuthSignIn(ctx, &tg.AuthSignInRequest{
			PhoneNumber:   phone,
			PhoneCodeHash: phoneCodeHash,
			PhoneCode:     req.Code,
		})
		if err != nil {
			return c.classifyError(err)
		}

		switch authResult := signInResult.(type) {
		case *tg.AuthAuthorization:
			user := authResult.User
			if u, ok := user.(*tg.User); ok {
				profile := &AccountProfile{
					UserID:       u.ID,
					Phone:        u.Phone,
					Username:     u.Username,
					FirstName:    u.FirstName,
					LastName:     u.LastName,
					IsPremium:    u.Premium,
					IsRestricted: u.Restricted,
					IsScam:       u.Scam,
					IsFake:       u.Fake,
				}

				sessionData, exportErr := storage.ExportSession()
				if exportErr != nil {
					c.logger.Error("导出 session 失败", "error", exportErr)
				}

				result = &LoginStep{
					FlowID:      req.FlowID,
					State:       LoginStateAuthorized,
					Message:     "登录成功",
					Account:     profile,
					SessionData: sessionData,
				}
			} else {
				return &MTProtoError{Kind: ErrInternalError, Message: "无法解析用户信息"}
			}

		case *tg.AuthAuthorizationSignUpRequired:
			return &MTProtoError{Kind: ErrUnauthorized, Message: "该手机号未注册 Telegram 账号"}

		default:
			return &MTProtoError{Kind: ErrInternalError, Message: "未知的登录响应类型"}
		}

		return nil
	})

	if err != nil {
		if isSessionPasswordNeeded(err) {
			flow.State = LoginStateWaitingPassword
			flow.UpdatedAt = time.Now()
			c.flowStore.Update(ctx, *flow)

			return &LoginStep{
				FlowID:  req.FlowID,
				State:   LoginStateWaitingPassword,
				Message: "需要两步验证密码",
			}, nil
		}
		return nil, err
	}

	// 防御：确保不会返回 nil step + nil error
	if result == nil {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "验证码提交失败，未获取到授权结果"}
	}

	return result, nil
}

// SubmitPassword 提交 2FA 密码。
func (c *GotdClient) SubmitPassword(ctx context.Context, req SubmitPasswordRequest) (*LoginStep, error) {
	if c.flowStore == nil {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "FlowStore 未初始化"}
	}

	flow, err := c.flowStore.Get(ctx, req.FlowID)
	if err != nil {
		return nil, &MTProtoError{Kind: ErrLoginExpired, Message: "登录流程不存在或已过期"}
	}

	if flow.State != LoginStateWaitingPassword {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "流程状态不正确，当前需要验证码而非密码"}
	}

	client, storage := c.createClient(req.APIID, req.APIHash, req.FlowID)

	var result *LoginStep

	err = client.Run(ctx, func(ctx context.Context) error {
		passwordInfo, err := client.API().AccountGetPassword(ctx)
		if err != nil {
			return c.classifyError(err)
		}

		if !passwordInfo.HasPassword {
			return &MTProtoError{Kind: ErrInternalError, Message: "该账号未设置两步验证"}
		}

		inputPassword, err := auth.PasswordHash(
			[]byte(req.Password),
			passwordInfo.SRPID,
			passwordInfo.SRPB,
			passwordInfo.SecureRandom,
			passwordInfo.NewAlgo,
		)
		if err != nil {
			return &MTProtoError{Kind: ErrInternalError, Message: "计算密码哈希失败", Err: err}
		}

		authResult, err := client.API().AuthCheckPassword(ctx, inputPassword)
		if err != nil {
			return c.classifyError(err)
		}

		switch auth := authResult.(type) {
		case *tg.AuthAuthorization:
			user := auth.User
			if u, ok := user.(*tg.User); ok {
				profile := &AccountProfile{
					UserID:       u.ID,
					Phone:        u.Phone,
					Username:     u.Username,
					FirstName:    u.FirstName,
					LastName:     u.LastName,
					IsPremium:    u.Premium,
					IsRestricted: u.Restricted,
					IsScam:       u.Scam,
					IsFake:       u.Fake,
				}

				sessionData, exportErr := storage.ExportSession()
				if exportErr != nil {
					c.logger.Error("导出 session 失败", "error", exportErr)
				}

				result = &LoginStep{
					FlowID:      req.FlowID,
					State:       LoginStateAuthorized,
					Message:     "登录成功",
					Account:     profile,
					SessionData: sessionData,
				}
			} else {
				return &MTProtoError{Kind: ErrInternalError, Message: "无法解析用户信息"}
			}
		default:
			return &MTProtoError{Kind: ErrInternalError, Message: "未知的登录响应类型"}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 防御：确保不会返回 nil step + nil error
	if result == nil {
		return nil, &MTProtoError{Kind: ErrInternalError, Message: "密码提交失败，未获取到授权结果"}
	}

	return result, nil
}

// SyncProfile 同步账号资料（基于已保存的加密 Session）。
func (c *GotdClient) SyncProfile(ctx context.Context, req SyncProfileRequest) (*AccountProfile, error) {
	client, _ := c.createClientFromSession(req.APIID, req.APIHash, req.SessionFilePath)

	var result *AccountProfile

	err := client.Run(ctx, func(ctx context.Context) error {
		// 使用 users.getUsers 获取当前用户信息
		// 首先需要获取当前用户 ID
		self, err := client.API().UsersGetUsers(ctx, []tg.InputUserClass{
			&tg.InputUserSelf{},
		})
		if err != nil {
			return c.classifyError(err)
		}

		if len(self) == 0 {
			return &MTProtoError{Kind: ErrSessionInvalid, Message: "无法获取当前用户信息"}
		}

		user, ok := self[0].(*tg.User)
		if !ok {
			return &MTProtoError{Kind: ErrInternalError, Message: "无法解析用户信息"}
		}

		result = &AccountProfile{
			UserID:       user.ID,
			Phone:        user.Phone,
			Username:     user.Username,
			FirstName:    user.FirstName,
			LastName:     user.LastName,
			IsPremium:    user.Premium,
			IsRestricted: user.Restricted,
			IsScam:       user.Scam,
			IsFake:       user.Fake,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// CheckSession 检查 Session 是否有效。
func (c *GotdClient) CheckSession(ctx context.Context, req CheckSessionRequest) (*SessionStatus, error) {
	client, _ := c.createClientFromSession(req.APIID, req.APIHash, req.SessionFilePath)

	now := time.Now()
	result := &SessionStatus{
		CheckedAt: now,
	}

	err := client.Run(ctx, func(ctx context.Context) error {
		// 尝试获取当前用户信息来验证 session 有效性
		self, err := client.API().UsersGetUsers(ctx, []tg.InputUserClass{
			&tg.InputUserSelf{},
		})
		if err != nil {
			return c.classifyError(err)
		}

		if len(self) == 0 {
			result.Valid = false
			result.Status = "invalid"
			result.Message = "无法获取当前用户信息"
			return nil
		}

		if _, ok := self[0].(*tg.User); ok {
			result.Valid = true
			result.Status = "active"
			result.Message = "Session 有效"
		} else {
			result.Valid = false
			result.Status = "invalid"
			result.Message = "用户信息解析失败"
		}

		return nil
	})

	if err != nil {
		// 检查是否是 session 相关错误
		errStr := err.Error()
		if strings.Contains(errStr, "AUTH_KEY_INVALID") || strings.Contains(errStr, "SESSION_REVOKED") ||
			strings.Contains(errStr, "USER_DEACTIVATED") || strings.Contains(errStr, "SESSION_EXPIRED") {
			result.Valid = false
			result.Status = "invalid"
			result.Message = "Session 已失效"
			return result, nil
		}

		// FLOOD_WAIT 直接返回错误
		if _, ok := err.(*FloodWaitError); ok {
			return nil, err
		}

		// 其他错误
		result.Valid = false
		result.Status = "error"
		result.Message = "检测失败"
		return result, nil
	}

	return result, nil
}

// Logout 从 Telegram 服务器登出。
// 使用已保存的加密 Session 连接 MTProto 并调用 auth.logOut。
// 不直接删除本地 Session 文件，由 AccountService 负责。
func (c *GotdClient) Logout(ctx context.Context, req LogoutRequest) error {
	client, _ := c.createClientFromSession(req.APIID, req.APIHash, req.SessionFilePath)

	err := client.Run(ctx, func(ctx context.Context) error {
		_, err := client.API().AuthLogOut(ctx)
		if err != nil {
			return c.classifyError(err)
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// classifyError 分类 gotd/td 错误。
func (c *GotdClient) classifyError(err error) error {
	if err == nil {
		return nil
	}

	// 检查 context 错误（超时/取消）
	if err == context.DeadlineExceeded {
		return &MTProtoError{Kind: ErrTelegramTimeout, Message: "连接 Telegram 超时，请检查 API 网络代理配置"}
	}
	if err == context.Canceled {
		return &MTProtoError{Kind: ErrNetworkError, Message: "连接已取消"}
	}

	// 检查代理相关错误
	if isProxyError(err) {
		return classifyProxyError(err)
	}

	errStr := err.Error()

	if strings.Contains(errStr, "FLOOD_WAIT") {
		waitSeconds := parseFloodWaitSeconds(errStr)
		return &FloodWaitError{
			Wait:    time.Duration(waitSeconds) * time.Second,
			Message: fmt.Sprintf("请等待 %d 秒后重试", waitSeconds),
		}
	}

	switch {
	case strings.Contains(errStr, "PHONE_NUMBER_BANNED"):
		return &MTProtoError{Kind: ErrUnauthorized, Message: "该手机号已被封禁"}
	case strings.Contains(errStr, "PHONE_NUMBER_INVALID"):
		return &MTProtoError{Kind: ErrInvalidPhone, Message: "手机号无效"}
	case strings.Contains(errStr, "PHONE_CODE_EXPIRED"):
		return &MTProtoError{Kind: ErrLoginCodeExpired, Message: "验证码已过期"}
	case strings.Contains(errStr, "PHONE_CODE_INVALID"):
		return &MTProtoError{Kind: ErrLoginCodeInvalid, Message: "验证码不正确"}
	case strings.Contains(errStr, "SESSION_PASSWORD_NEEDED"):
		return &MTProtoError{Kind: ErrLoginPasswordRequired, Message: "需要两步验证密码"}
	case strings.Contains(errStr, "PASSWORD_HASH_INVALID"):
		return &MTProtoError{Kind: ErrLoginPasswordInvalid, Message: "两步验证密码不正确"}
	case strings.Contains(errStr, "SRP_ID_INVALID"), strings.Contains(errStr, "SRP_PASSWORD_CHANGED"):
		return &MTProtoError{Kind: ErrLoginPasswordInvalid, Message: "密码验证失败，请重试"}
	case strings.Contains(errStr, "API_ID_INVALID"):
		return &MTProtoError{Kind: ErrCredentialDisabled, Message: "API ID 无效"}
	case strings.Contains(errStr, "API_ID_PUBLISHED_FLOOD"):
		return &MTProtoError{Kind: ErrCredentialDisabled, Message: "API ID 已被限制使用"}
	case strings.Contains(errStr, "AUTH_RESTART"):
		return &MTProtoError{Kind: ErrSessionInvalid, Message: "登录流程已过期，请重新开始"}
	case strings.Contains(errStr, "PHONE_NUMBER_UNOCCUPIED"):
		return &MTProtoError{Kind: ErrUnauthorized, Message: "该手机号未注册 Telegram 账号"}
	case strings.Contains(errStr, "AUTH_KEY_INVALID"), strings.Contains(errStr, "SESSION_REVOKED"),
		strings.Contains(errStr, "SESSION_EXPIRED"):
		return &MTProtoError{Kind: ErrSessionInvalid, Message: "Session 已失效，请重新登录该账号"}
	case strings.Contains(errStr, "USER_DEACTIVATED"):
		return &MTProtoError{Kind: ErrUnauthorized, Message: "账号已被停用"}
	}

	return &MTProtoError{Kind: ErrNetworkError, Message: "网络错误", Err: err}
}

// isSessionPasswordNeeded 检查错误是否是 SESSION_PASSWORD_NEEDED。
func isSessionPasswordNeeded(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "SESSION_PASSWORD_NEEDED")
}

// parseFloodWaitSeconds 从错误消息中解析 FLOOD_WAIT 秒数。
func parseFloodWaitSeconds(errStr string) int {
	parts := strings.Split(errStr, "FLOOD_WAIT_")
	if len(parts) > 1 {
		seconds := 0
		fmt.Sscanf(parts[1], "%d", &seconds)
		if seconds > 0 {
			return seconds
		}
	}
	return 60
}

// maskPhone 脱敏手机号。
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:3] + "***" + phone[len(phone)-2:]
}

// isProxyError 检查错误是否与代理相关。
func isProxyError(err error) bool {
	if err == nil {
		return false
	}
	// 检查是否是 net.OpError 且涉及代理连接
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		// 连接被拒绝、超时等都可能是代理问题
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "proxy") ||
		strings.Contains(errStr, "SOCKS") ||
		strings.Contains(errStr, "CONNECT") ||
		strings.Contains(errStr, "407") // Proxy Authentication Required
}

// classifyProxyError 分类代理错误。
func classifyProxyError(err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "auth") || strings.Contains(errStr, "407") {
		return &MTProtoError{Kind: ErrProxyAuthFailed, Message: "代理认证失败，请检查用户名和密码", Err: err}
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return &MTProtoError{Kind: ErrProxyConnectFailed, Message: "连接代理服务器超时，请检查代理地址和端口", Err: err}
	}
	if strings.Contains(errStr, "refused") || strings.Contains(errStr, "no route") {
		return &MTProtoError{Kind: ErrProxyConnectFailed, Message: "无法连接到代理服务器，请检查代理地址和端口", Err: err}
	}

	return &MTProtoError{Kind: ErrProxyConnectFailed, Message: "代理连接失败，请检查代理配置", Err: err}
}

// 确保 GotdClient 实现 Client 接口。
var _ Client = (*GotdClient)(nil)
