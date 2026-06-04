package account

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/mtproto"
	"github.com/user/atria/internal/security"

	"gorm.io/gorm"
)

// Service 是账号管理业务服务。
type Service struct {
	db           *gorm.DB
	key          []byte
	sessionStore mtproto.SessionStore
	client       mtproto.Client // MTProto 客户端接口
}

// NewService 创建账号服务。
func NewService(db *gorm.DB, key []byte, sessionStore mtproto.SessionStore, client mtproto.Client) *Service {
	return &Service{db: db, key: key, sessionStore: sessionStore, client: client}
}

// ListAccounts 列出所有账号。
func (s *Service) ListAccounts(ctx context.Context) ([]model.TelegramAccount, error) {
	var accounts []model.TelegramAccount
	err := s.db.Preload("APICredential").Preload("Session").
		Order("id DESC").Find(&accounts).Error
	return accounts, err
}

// GetAccount 获取账号详情。
func (s *Service) GetAccount(ctx context.Context, id uint) (*model.TelegramAccount, error) {
	var account model.TelegramAccount
	err := s.db.Preload("APICredential").Preload("Session").
		First(&account, id).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// findByUserIDOrFingerprint 按 user_id 或 phone_fingerprint 查找已有账号。
func (s *Service) findByUserIDOrFingerprint(userID int64, phoneFingerprint string) (*model.TelegramAccount, error) {
	var account model.TelegramAccount

	// 优先按 user_id 查找
	err := s.db.Where("user_id = ?", userID).First(&account).Error
	if err == nil {
		return &account, nil
	}

	// 再按 phone_fingerprint 查找
	if phoneFingerprint != "" {
		err = s.db.Where("phone_fingerprint = ?", phoneFingerprint).First(&account).Error
		if err == nil {
			return &account, nil
		}
	}

	return nil, gorm.ErrRecordNotFound
}

// CompleteLoginInput 是登录完成后的输入。
type CompleteLoginInput struct {
	APICredentialID uint
	Profile         *mtproto.AccountProfile
	SessionData     []byte
	ActorID         uint
	IP              string
	UserAgent       string
}

// CompleteLogin 完成登录流程，创建或更新账号。
func (s *Service) CompleteLogin(ctx context.Context, input CompleteLoginInput) (*model.TelegramAccount, error) {
	if input.Profile == nil {
		return nil, fmt.Errorf("账号资料不能为空")
	}

	phoneEncrypted, phoneFingerprint, err := security.EncryptPhone(s.key, input.Profile.Phone)
	if err != nil {
		return nil, fmt.Errorf("加密手机号失败: %w", err)
	}

	now := time.Now()

	existing, err := s.findByUserIDOrFingerprint(input.Profile.UserID, phoneFingerprint)

	var account *model.TelegramAccount
	var isUpdate bool

	if err == nil && existing != nil {
		account = existing
		isUpdate = true

		account.APICredentialID = input.APICredentialID
		account.PhoneEncrypted = phoneEncrypted
		account.PhoneFingerprint = phoneFingerprint
		account.Username = input.Profile.Username
		account.FirstName = input.Profile.FirstName
		account.LastName = input.Profile.LastName
		account.DisplayName = buildDisplayName(input.Profile.FirstName, input.Profile.LastName, input.Profile.Username)
		account.Status = model.TelegramAccountStatusActive
		account.IsPremium = input.Profile.IsPremium
		account.IsRestricted = input.Profile.IsRestricted
		account.IsScam = input.Profile.IsScam
		account.IsFake = input.Profile.IsFake
		account.UpdatedAt = now

		if err := s.db.Save(account).Error; err != nil {
			return nil, fmt.Errorf("更新账号失败: %w", err)
		}
	} else {
		account = &model.TelegramAccount{
			APICredentialID:  input.APICredentialID,
			UserID:           input.Profile.UserID,
			PhoneEncrypted:   phoneEncrypted,
			PhoneFingerprint: phoneFingerprint,
			Username:         input.Profile.Username,
			FirstName:        input.Profile.FirstName,
			LastName:         input.Profile.LastName,
			DisplayName:      buildDisplayName(input.Profile.FirstName, input.Profile.LastName, input.Profile.Username),
			Status:           model.TelegramAccountStatusActive,
			IsPremium:        input.Profile.IsPremium,
			IsRestricted:     input.Profile.IsRestricted,
			IsScam:           input.Profile.IsScam,
			IsFake:           input.Profile.IsFake,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := s.db.Create(account).Error; err != nil {
			return nil, fmt.Errorf("创建账号失败: %w", err)
		}
	}

	// 保存 Session 文件
	if len(input.SessionData) > 0 {
		sessionInfo, err := s.sessionStore.Save(account.ID, input.SessionData)
		if err != nil {
			return nil, fmt.Errorf("保存 Session 文件失败: %w", err)
		}

		var session model.AccountSession
		result := s.db.Where("telegram_account_id = ?", account.ID).First(&session)

		if result.Error == gorm.ErrRecordNotFound {
			session = model.AccountSession{
				TelegramAccountID:  account.ID,
				SessionFilePath:    sessionInfo.Path,
				SessionFingerprint: sessionInfo.Fingerprint,
				EncryptionVersion:  1,
				Status:             "active",
				LastVerifiedAt:     &now,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			if err := s.db.Create(&session).Error; err != nil {
				return nil, fmt.Errorf("创建 Session 记录失败: %w", err)
			}
		} else if result.Error == nil {
			session.SessionFilePath = sessionInfo.Path
			session.SessionFingerprint = sessionInfo.Fingerprint
			session.EncryptionVersion = 1
			session.Status = "active"
			session.LastVerifiedAt = &now
			session.UpdatedAt = now
			if err := s.db.Save(&session).Error; err != nil {
				return nil, fmt.Errorf("更新 Session 记录失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("查询 Session 记录失败: %w", result.Error)
		}

		account.Session = &session

		audit.Log(ctx, s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", input.ActorID),
			Action:       "account.session_saved",
			ResourceType: "telegram_account",
			ResourceID:   fmt.Sprintf("%d", account.ID),
			RiskLevel:    "medium",
			IP:           input.IP,
			UserAgent:    input.UserAgent,
			Message:      "Session 已保存",
			Metadata: map[string]any{
				"user_id":           account.UserID,
				"api_credential_id": account.APICredentialID,
			},
		})
	}

	action := "account.created"
	message := "创建 Telegram 账号"
	if isUpdate {
		action = "account.session_refreshed"
		message = "刷新 Telegram 账号 Session"
	}

	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", input.ActorID),
		Action:       action,
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", account.ID),
		RiskLevel:    "medium",
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Message:      message,
		Metadata: map[string]any{
			"user_id":           account.UserID,
			"username":          account.Username,
			"api_credential_id": account.APICredentialID,
		},
	})

	return account, nil
}

// SyncProfileInput 是同步账号资料的输入。
type SyncProfileInput struct {
	AccountID uint
	ActorID   uint
	IP        string
	UserAgent string
}

// SyncProfileResult 是同步账号资料的结果。
type SyncProfileResult struct {
	Account *model.TelegramAccount
	Updated bool
}

// SyncProfile 同步账号资料。
func (s *Service) SyncProfile(ctx context.Context, input SyncProfileInput) (*SyncProfileResult, error) {
	// 查询账号
	account, err := s.GetAccount(ctx, input.AccountID)
	if err != nil {
		return nil, fmt.Errorf("账号不存在")
	}

	// 查询绑定 API 凭据
	var cred model.APICredential
	if err := s.db.First(&cred, account.APICredentialID).Error; err != nil {
		return nil, fmt.Errorf("绑定的 API 凭据不存在")
	}

	// 检查凭据状态
	if cred.Status != model.APICredentialStatusEnabled {
		return nil, fmt.Errorf("当前 API 凭据已禁用，请重新选择或启用凭据")
	}

	// 检查 Session 文件
	if account.Session == nil {
		return nil, fmt.Errorf("该账号没有 Session 记录")
	}

	if !s.sessionStore.Exists(account.Session.SessionFilePath) {
		return nil, fmt.Errorf("Session 文件不存在，请重新登录该账号")
	}

	// 解密 api_hash（仅短暂存在于内存）
	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		return nil, fmt.Errorf("解密凭据失败")
	}

	// 调用 MTProto 同步资料
	profile, err := s.client.SyncProfile(ctx, mtproto.SyncProfileRequest{
		APICredentialID: cred.ID,
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		AccountID:       account.ID,
		SessionFilePath: account.Session.SessionFilePath,
	})

	if err != nil {
		// 写入同步失败审计日志
		errKind := mtproto.ClassifyError(err)
		audit.Log(ctx, s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", input.ActorID),
			Action:       "account.profile_sync_failed",
			ResourceType: "telegram_account",
			ResourceID:   fmt.Sprintf("%d", account.ID),
			RiskLevel:    "medium",
			IP:           input.IP,
			UserAgent:    input.UserAgent,
			Message:      "账号资料同步失败",
			Metadata: map[string]any{
				"account_id":        account.ID,
				"api_credential_id": account.APICredentialID,
				"error_kind":        string(errKind),
			},
		})

		// 返回友好错误
		return nil, classifySyncError(err)
	}

	// 更新账号资料
	now := time.Now()
	account.Username = profile.Username
	account.FirstName = profile.FirstName
	account.LastName = profile.LastName
	account.DisplayName = buildDisplayName(profile.FirstName, profile.LastName, profile.Username)
	account.IsPremium = profile.IsPremium
	account.IsRestricted = profile.IsRestricted
	account.IsScam = profile.IsScam
	account.IsFake = profile.IsFake
	account.LastSyncAt = &now
	account.UpdatedAt = now

	// 更新手机号（如果 Telegram 返回了新手机号）
	if profile.Phone != "" {
		phoneEncrypted, phoneFingerprint, err := security.EncryptPhone(s.key, profile.Phone)
		if err == nil {
			account.PhoneEncrypted = phoneEncrypted
			account.PhoneFingerprint = phoneFingerprint
		}
	}

	if err := s.db.Save(account).Error; err != nil {
		return nil, fmt.Errorf("更新账号资料失败: %w", err)
	}

	// 写入同步快照摘要
	summary := fmt.Sprintf("同步于 %s，用户名: %s", now.Format("2006-01-02 15:04:05"), account.Username)
	snapshot := model.AccountSyncSnapshot{
		TelegramAccountID: account.ID,
		SnapshotType:      model.SyncSnapshotTypeProfile,
		PayloadSummary:    summary,
		ItemCount:         1,
		CreatedAt:         now,
	}
	s.db.Create(&snapshot)

	// 写入同步成功审计日志
	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", input.ActorID),
		Action:       "account.profile_synced",
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", account.ID),
		RiskLevel:    "low",
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Message:      "账号资料同步成功",
		Metadata: map[string]any{
			"account_id":        account.ID,
			"api_credential_id": account.APICredentialID,
			"username":          account.Username,
		},
	})

	return &SyncProfileResult{
		Account: account,
		Updated: true,
	}, nil
}

// CheckSessionInput 是检测 Session 状态的输入。
type CheckSessionInput struct {
	AccountID uint
	ActorID   uint
	IP        string
	UserAgent string
}

// CheckSessionResult 是检测 Session 状态的结果。
type CheckSessionResult struct {
	SessionStatus string
	Valid         bool
	Message       string
}

// CheckSession 检测 Session 状态。
func (s *Service) CheckSession(ctx context.Context, input CheckSessionInput) (*CheckSessionResult, error) {
	// 查询账号
	account, err := s.GetAccount(ctx, input.AccountID)
	if err != nil {
		return nil, fmt.Errorf("账号不存在")
	}

	// 查询 account_session
	if account.Session == nil {
		return nil, fmt.Errorf("该账号没有 Session 记录")
	}

	// 查询绑定 API 凭据
	var cred model.APICredential
	if err := s.db.First(&cred, account.APICredentialID).Error; err != nil {
		return nil, fmt.Errorf("绑定的 API 凭据不存在")
	}

	// 检查凭据状态
	if cred.Status != model.APICredentialStatusEnabled {
		return nil, fmt.Errorf("当前 API 凭据已禁用，请重新选择或启用凭据")
	}

	// 检查 Session 文件
	if !s.sessionStore.Exists(account.Session.SessionFilePath) {
		// 文件不存在，更新状态
		account.Session.Status = "invalid"
		account.Session.UpdatedAt = time.Now()
		s.db.Save(account.Session)

		return &CheckSessionResult{
			SessionStatus: "invalid",
			Valid:         false,
			Message:       "Session 文件不存在",
		}, nil
	}

	// 解密 api_hash
	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		return nil, fmt.Errorf("解密凭据失败")
	}

	// 调用 MTProto 检测 Session
	status, err := s.client.CheckSession(ctx, mtproto.CheckSessionRequest{
		APICredentialID: cred.ID,
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		AccountID:       account.ID,
		SessionFilePath: account.Session.SessionFilePath,
	})

	if err != nil {
		// 写入检测失败审计日志
		errKind := mtproto.ClassifyError(err)
		audit.Log(ctx, s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", input.ActorID),
			Action:       "account.session_check_failed",
			ResourceType: "telegram_account",
			ResourceID:   fmt.Sprintf("%d", account.ID),
			RiskLevel:    "medium",
			IP:           input.IP,
			UserAgent:    input.UserAgent,
			Message:      "Session 状态检测失败",
			Metadata: map[string]any{
				"account_id":        account.ID,
				"api_credential_id": account.APICredentialID,
				"error_kind":        string(errKind),
			},
		})

		return nil, classifySyncError(err)
	}

	// 更新 Session 状态
	now := time.Now()
	account.Session.Status = status.Status
	account.Session.LastVerifiedAt = &now
	account.Session.UpdatedAt = now

	if err := s.db.Save(account.Session).Error; err != nil {
		slog.Error("更新 Session 状态失败", "error", err)
	}

	// 如果 Session 无效，更新账号状态
	if !status.Valid {
		account.Status = model.TelegramAccountStatusLoggedOut
		account.UpdatedAt = now
		s.db.Save(account)
	}

	// 写入检测成功审计日志
	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", input.ActorID),
		Action:       "account.session_checked",
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", account.ID),
		RiskLevel:    "low",
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Message:      fmt.Sprintf("Session 状态: %s", status.Status),
		Metadata: map[string]any{
			"account_id":        account.ID,
			"api_credential_id": account.APICredentialID,
			"session_status":    status.Status,
			"valid":             status.Valid,
		},
	})

	return &CheckSessionResult{
		SessionStatus: status.Status,
		Valid:         status.Valid,
		Message:       status.Message,
	}, nil
}

// MarkSessionStatus 更新 Session 状态。
func (s *Service) MarkSessionStatus(ctx context.Context, accountID uint, status string) error {
	var session model.AccountSession
	if err := s.db.Where("telegram_account_id = ?", accountID).First(&session).Error; err != nil {
		return fmt.Errorf("Session 记录不存在")
	}

	session.Status = status
	session.UpdatedAt = time.Now()

	return s.db.Save(&session).Error
}

// DeleteAccountSession 删除账号的 Session 文件和记录。
func (s *Service) DeleteAccountSession(ctx context.Context, accountID uint) error {
	var session model.AccountSession
	if err := s.db.Where("telegram_account_id = ?", accountID).First(&session).Error; err != nil {
		return fmt.Errorf("Session 记录不存在")
	}

	if s.sessionStore.Exists(session.SessionFilePath) {
		if err := s.sessionStore.Delete(session.SessionFilePath); err != nil {
			slog.Error("删除 Session 文件失败", "error", err, "path", session.SessionFilePath)
		}
	}

	session.Status = "deleted"
	session.UpdatedAt = time.Now()
	return s.db.Save(&session).Error
}

// LogoutAccount 本地登出账号。
func (s *Service) LogoutAccount(ctx context.Context, actorID, accountID uint) error {
	account, err := s.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("账号不存在")
	}

	if err := s.DeleteAccountSession(ctx, accountID); err != nil {
		slog.Error("删除 Session 失败", "error", err)
	}

	account.Status = model.TelegramAccountStatusLoggedOut
	account.UpdatedAt = time.Now()
	if err := s.db.Save(account).Error; err != nil {
		return fmt.Errorf("更新账号状态失败: %w", err)
	}

	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", actorID),
		Action:       "account.logged_out",
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", accountID),
		RiskLevel:    "medium",
		Message:      "账号本地登出",
		Metadata: map[string]any{
			"user_id": account.UserID,
		},
	})

	return nil
}

// RemoteLogoutInput 是远端 Logout 的输入。
type RemoteLogoutInput struct {
	AccountID uint
	ActorID   uint
	IP        string
	UserAgent string
}

// RemoteLogout 远端注销账号 Session。
// 调用 MTProto logout，成功后删除本地 Session 文件。
func (s *Service) RemoteLogout(ctx context.Context, input RemoteLogoutInput) error {
	// 查询账号
	account, err := s.GetAccount(ctx, input.AccountID)
	if err != nil {
		return fmt.Errorf("账号不存在")
	}

	// 查询 account_session
	if account.Session == nil {
		return fmt.Errorf("该账号没有 Session 记录")
	}

	// 查询绑定 API 凭据
	var cred model.APICredential
	if err := s.db.First(&cred, account.APICredentialID).Error; err != nil {
		return fmt.Errorf("绑定的 API 凭据不存在")
	}

	// 检查凭据状态
	if cred.Status != model.APICredentialStatusEnabled {
		return fmt.Errorf("当前 API 凭据已禁用，请重新选择或启用凭据")
	}

	// 检查 Session 文件
	if !s.sessionStore.Exists(account.Session.SessionFilePath) {
		return fmt.Errorf("本地 Session 文件不存在，请重新登录该账号")
	}

	// 解密 api_hash
	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		slog.Error("解密 api_hash 失败", "error", err)
		return fmt.Errorf("解密凭据失败")
	}

	// 调用 MTProto 远端 Logout
	err = s.client.Logout(ctx, mtproto.LogoutRequest{
		APICredentialID: cred.ID,
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		AccountID:       account.ID,
		SessionFilePath: account.Session.SessionFilePath,
	})

	if err != nil {
		// 写入失败审计日志
		errKind := mtproto.ClassifyError(err)
		audit.Log(ctx, s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", input.ActorID),
			Action:       "account.remote_logout_failed",
			ResourceType: "telegram_account",
			ResourceID:   fmt.Sprintf("%d", account.ID),
			RiskLevel:    "medium",
			IP:           input.IP,
			UserAgent:    input.UserAgent,
			Message:      "远端 Logout 失败",
			Metadata: map[string]any{
				"account_id":        account.ID,
				"api_credential_id": account.APICredentialID,
				"error_kind":        string(errKind),
			},
		})

		return classifySyncError(err)
	}

	// 远端 Logout 成功，删除本地 Session 文件
	if s.sessionStore.Exists(account.Session.SessionFilePath) {
		if err := s.sessionStore.Delete(account.Session.SessionFilePath); err != nil {
			slog.Error("删除本地 Session 文件失败", "error", err)
		}
	}

	// 更新 account_sessions 状态
	account.Session.Status = "deleted"
	account.Session.UpdatedAt = time.Now()
	s.db.Save(account.Session)

	// 更新 telegram_accounts 状态
	account.Status = model.TelegramAccountStatusLoggedOut
	account.UpdatedAt = time.Now()
	s.db.Save(account)

	// 写入成功审计日志
	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", input.ActorID),
		Action:       "account.remote_logout",
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", account.ID),
		RiskLevel:    "medium",
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Message:      "远端 Logout 成功",
		Metadata: map[string]any{
			"account_id":        account.ID,
			"api_credential_id": account.APICredentialID,
		},
	})

	return nil
}

// DeleteLocalSessionInput 是本地删除 Session 的输入。
type DeleteLocalSessionInput struct {
	AccountID uint
	ActorID   uint
	IP        string
	UserAgent string
}

// DeleteLocalSession 仅删除本地 Session 文件，不调用 Telegram 远端。
func (s *Service) DeleteLocalSession(ctx context.Context, input DeleteLocalSessionInput) error {
	// 查询账号
	account, err := s.GetAccount(ctx, input.AccountID)
	if err != nil {
		return fmt.Errorf("账号不存在")
	}

	// 查询 account_session
	if account.Session == nil {
		return fmt.Errorf("该账号没有 Session 记录")
	}

	sessionPath := account.Session.SessionFilePath

	// 删除本地 Session 文件
	if s.sessionStore.Exists(sessionPath) {
		if err := s.sessionStore.Delete(sessionPath); err != nil {
			slog.Error("删除本地 Session 文件失败", "error", err)

			// 写入失败审计日志
			audit.Log(ctx, s.db, audit.Event{
				ActorType:    "admin",
				ActorID:      fmt.Sprintf("%d", input.ActorID),
				Action:       "account.local_session_delete_failed",
				ResourceType: "telegram_account",
				ResourceID:   fmt.Sprintf("%d", account.ID),
				RiskLevel:    "medium",
				IP:           input.IP,
				UserAgent:    input.UserAgent,
				Message:      "本地删除 Session 文件失败",
				Metadata: map[string]any{
					"account_id": account.ID,
				},
			})

			return fmt.Errorf("删除 Session 文件失败: %w", err)
		}
	}

	// 更新 account_sessions 状态
	account.Session.Status = "deleted"
	account.Session.UpdatedAt = time.Now()
	s.db.Save(account.Session)

	// 更新 telegram_accounts 状态
	account.Status = model.TelegramAccountStatusLoggedOut
	account.UpdatedAt = time.Now()
	s.db.Save(account)

	// 写入成功审计日志
	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", input.ActorID),
		Action:       "account.local_session_deleted",
		ResourceType: "telegram_account",
		ResourceID:   fmt.Sprintf("%d", account.ID),
		RiskLevel:    "medium",
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Message:      "本地 Session 已删除",
		Metadata: map[string]any{
			"account_id":        account.ID,
			"api_credential_id": account.APICredentialID,
		},
	})

	return nil
}

// HandleSessionInvalid 处理 Session 失效。
func (s *Service) HandleSessionInvalid(ctx context.Context, accountID uint, reason string) error {
	account, err := s.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("账号不存在")
	}

	// 更新 account_sessions 状态
	if account.Session != nil {
		account.Session.Status = "invalid"
		account.Session.UpdatedAt = time.Now()
		s.db.Save(account.Session)
	}

	// 更新 telegram_accounts 状态
	account.Status = model.TelegramAccountStatusLoggedOut
	account.UpdatedAt = time.Now()
	if err := s.db.Save(account).Error; err != nil {
		return fmt.Errorf("更新账号状态失败: %w", err)
	}

	return nil
}

// CleanupExpiredLoginFlowsInput 是清理过期登录流程的输入。
type CleanupExpiredLoginFlowsInput struct {
	ActorID   uint
	IP        string
	UserAgent string
}

// CleanupExpiredLoginFlows 清理过期的 LoginFlow 和对应临时 Session。
func (s *Service) CleanupExpiredLoginFlows(ctx context.Context, flowStore mtproto.FlowStore, sessionDir string, input CleanupExpiredLoginFlowsInput) (int, error) {
	// MemoryFlowStore 支持 CleanupExpired
	type cleanupable interface {
		CleanupExpired() int
	}

	if c, ok := flowStore.(cleanupable); ok {
		count := c.CleanupExpired()

		if count > 0 {
			audit.Log(ctx, s.db, audit.Event{
				ActorType:    "admin",
				ActorID:      fmt.Sprintf("%d", input.ActorID),
				Action:       "account.login_flow_cleaned",
				ResourceType: "login_flow",
				ResourceID:   "0",
				RiskLevel:    "low",
				IP:           input.IP,
				UserAgent:    input.UserAgent,
				Message:      fmt.Sprintf("清理了 %d 个过期登录流程", count),
				Metadata: map[string]any{
					"cleaned_count": count,
				},
			})
		}

		return count, nil
	}

	return 0, nil
}

// buildDisplayName 构建显示名称。
func buildDisplayName(firstName, lastName, username string) string {
	if firstName != "" && lastName != "" {
		return firstName + " " + lastName
	}
	if firstName != "" {
		return firstName
	}
	if lastName != "" {
		return lastName
	}
	if username != "" {
		return "@" + username
	}
	return "未知用户"
}

// classifySyncError 将同步/检测错误转换为用户友好消息。
func classifySyncError(err error) error {
	if err == nil {
		return nil
	}

	errKind := mtproto.ClassifyError(err)
	switch errKind {
	case mtproto.ErrFloodWait:
		if floodErr, ok := err.(*mtproto.FloodWaitError); ok {
			return fmt.Errorf("请求过于频繁，请等待 %s 后重试", floodErr.Wait)
		}
		return fmt.Errorf("请求过于频繁，请稍后重试")
	case mtproto.ErrSessionInvalid:
		return fmt.Errorf("Session 已失效，请重新登录该账号")
	case mtproto.ErrUnauthorized:
		return fmt.Errorf("账号已被封禁或限制")
	case mtproto.ErrCredentialDisabled:
		return fmt.Errorf("当前 API 凭据已禁用，请重新选择或启用凭据")
	case mtproto.ErrNetworkError:
		return fmt.Errorf("网络异常，请稍后重试")
	default:
		return fmt.Errorf("操作失败，请稍后重试")
	}
}
