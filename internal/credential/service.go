package credential

import (
	"fmt"
	"strings"
	"time"

	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/security"

	"gorm.io/gorm"
)

// Service 是 API 凭据业务服务。
type Service struct {
	db  *gorm.DB
	key []byte
}

// NewService 创建凭据服务。
func NewService(db *gorm.DB, key []byte) *Service {
	return &Service{db: db, key: key}
}

// List 获取所有未删除的凭据。
func (s *Service) List() ([]model.APICredential, error) {
	var credentials []model.APICredential
	err := s.db.Order("id DESC").Find(&credentials).Error
	return credentials, err
}

// ListEnabled 获取所有启用且未删除的凭据。
func (s *Service) ListEnabled() ([]model.APICredential, error) {
	var credentials []model.APICredential
	err := s.db.Where("status = ?", model.APICredentialStatusEnabled).
		Order("is_default DESC, id DESC").Find(&credentials).Error
	return credentials, err
}

// GetDefault 获取系统默认凭据。
func (s *Service) GetDefault() (*model.APICredential, error) {
	var cred model.APICredential
	err := s.db.Where("is_default = ? AND status = ?", true, model.APICredentialStatusEnabled).
		First(&cred).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// GetSystemAPIKey 获取系统 API Key。
// 优先返回 is_default=true 且 enabled 的记录。
// 如果没有默认记录但有启用记录，自动将第一条启用记录设为默认。
// 如果没有任何启用记录，返回 nil。
func (s *Service) GetSystemAPIKey() (*model.APICredential, error) {
	// 先尝试获取默认记录
	cred, err := s.GetDefault()
	if err == nil && cred != nil {
		return cred, nil
	}

	// 没有默认记录，找第一条启用记录
	var enabled model.APICredential
	err = s.db.Where("status = ?", model.APICredentialStatusEnabled).
		Order("id ASC").First(&enabled).Error
	if err != nil {
		return nil, nil // 没有启用记录
	}

	// 自动设为默认
	s.db.Model(&model.APICredential{}).Where("id = ?", enabled.ID).Update("is_default", true)
	enabled.IsDefault = true

	return &enabled, nil
}

// SetDefault 设置指定凭据为默认。
func (s *Service) SetDefault(id uint) error {
	// 先取消所有默认
	if err := s.db.Model(&model.APICredential{}).Where("is_default = ?", true).
		Update("is_default", false).Error; err != nil {
		return fmt.Errorf("取消默认凭据失败: %w", err)
	}

	// 设置新默认
	if err := s.db.Model(&model.APICredential{}).Where("id = ?", id).
		Update("is_default", true).Error; err != nil {
		return fmt.Errorf("设置默认凭据失败: %w", err)
	}

	return nil
}

// EnsureDefault 确保存在默认凭据。如果没有任何默认凭据，将第一个启用凭据设为默认。
func (s *Service) EnsureDefault() error {
	// 检查是否已有默认
	var defaultCount int64
	s.db.Model(&model.APICredential{}).Where("is_default = ? AND status = ?", true, model.APICredentialStatusEnabled).Count(&defaultCount)
	if defaultCount > 0 {
		return nil
	}

	// 找第一个启用凭据
	var cred model.APICredential
	err := s.db.Where("status = ?", model.APICredentialStatusEnabled).Order("id ASC").First(&cred).Error
	if err != nil {
		return nil // 没有启用凭据，不需要设置
	}

	return s.SetDefault(cred.ID)
}

// GetByID 根据 ID 获取凭据（未删除）。
func (s *Service) GetByID(id uint) (*model.APICredential, error) {
	var cred model.APICredential
	err := s.db.First(&cred, id).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// CreateInput 是创建凭据的输入。
type CreateInput struct {
	DisplayName string
	APIID       string
	APIHash     string
	Status      string
	RiskPolicy  string
}

// Create 创建新凭据。
func (s *Service) Create(input CreateInput) (*model.APICredential, error) {
	// 校验
	displayName := strings.TrimSpace(input.DisplayName)
	if err := ValidateDisplayName(displayName); err != nil {
		return nil, err
	}

	apiID, err := ValidateAPIID(input.APIID)
	if err != nil {
		return nil, err
	}

	apiHash := strings.TrimSpace(input.APIHash)
	if err := ValidateAPIHash(apiHash); err != nil {
		return nil, err
	}

	status, err := ValidateStatus(input.Status)
	if err != nil {
		return nil, err
	}

	riskPolicy, err := ValidateRiskPolicy(input.RiskPolicy)
	if err != nil {
		return nil, err
	}

	// 加密 api_hash
	encrypted, fingerprint, err := security.EncryptAPIHash(s.key, apiHash)
	if err != nil {
		return nil, fmt.Errorf("加密 API Hash 失败: %w", err)
	}

	// 生成 hint
	hint := GenerateAPIHashHint(apiHash)

	// 检查是否是第一个凭据（自动设为默认）
	var count int64
	s.db.Model(&model.APICredential{}).Count(&count)
	isDefault := count == 0

	// 创建凭据
	now := time.Now()
	cred := &model.APICredential{
		DisplayName:        displayName,
		APIID:              apiID,
		EncryptedAPIHash:   encrypted,
		APIHashHint:        hint,
		APIHashFingerprint: fingerprint,
		IsDefault:          isDefault,
		Status:             status,
		RiskPolicy:         riskPolicy,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.db.Create(cred).Error; err != nil {
		return nil, fmt.Errorf("创建凭据失败: %w", err)
	}

	return cred, nil
}

// UpdateInput 是更新凭据的输入。
type UpdateInput struct {
	DisplayName string
	APIID       string
	APIHash     string // 为空时保持不变
	Status      string
	RiskPolicy  string
}

// Update 更新凭据。
func (s *Service) Update(id uint, input UpdateInput) (*model.APICredential, error) {
	cred, err := s.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("凭据不存在")
	}

	// 校验
	displayName := strings.TrimSpace(input.DisplayName)
	if err := ValidateDisplayName(displayName); err != nil {
		return nil, err
	}

	apiID, err := ValidateAPIID(input.APIID)
	if err != nil {
		return nil, err
	}

	status, err := ValidateStatus(input.Status)
	if err != nil {
		return nil, err
	}

	riskPolicy, err := ValidateRiskPolicy(input.RiskPolicy)
	if err != nil {
		return nil, err
	}

	// 更新基本字段
	cred.DisplayName = displayName
	cred.APIID = apiID
	cred.Status = status
	cred.RiskPolicy = riskPolicy
	cred.UpdatedAt = time.Now()

	// 如果提供了新的 api_hash，重新加密
	apiHash := strings.TrimSpace(input.APIHash)
	hashRotated := false
	if apiHash != "" {
		if err := ValidateAPIHash(apiHash); err != nil {
			return nil, err
		}

		encrypted, fingerprint, err := security.EncryptAPIHash(s.key, apiHash)
		if err != nil {
			return nil, fmt.Errorf("加密 API Hash 失败: %w", err)
		}

		cred.EncryptedAPIHash = encrypted
		cred.APIHashHint = GenerateAPIHashHint(apiHash)
		cred.APIHashFingerprint = fingerprint
		hashRotated = true
	}

	if err := s.db.Save(cred).Error; err != nil {
		return nil, fmt.Errorf("更新凭据失败: %w", err)
	}

	// 返回是否轮换了 hash
	if hashRotated {
		return cred, nil
	}

	return cred, nil
}

// UpdateStatus 更新凭据状态。
// 禁用默认凭据时，如果有其它启用凭据则自动切换默认；否则阻止禁用。
func (s *Service) UpdateStatus(id uint, statusStr string) error {
	status, err := ValidateStatus(statusStr)
	if err != nil {
		return err
	}

	cred, err := s.GetByID(id)
	if err != nil {
		return fmt.Errorf("凭据不存在")
	}

	// 禁用默认凭据时的保护逻辑
	if status == model.APICredentialStatusDisabled && cred.IsDefault {
		// 检查是否有其它启用凭据
		var otherEnabled int64
		s.db.Model(&model.APICredential{}).
			Where("id != ? AND status = ? AND deleted_at IS NULL", id, model.APICredentialStatusEnabled).
			Count(&otherEnabled)

		if otherEnabled == 0 {
			return fmt.Errorf("不能禁用唯一的默认凭据，请先创建其它启用凭据")
		}

		// 自动切换默认到第一个其它启用凭据
		var newDefault model.APICredential
		s.db.Where("id != ? AND status = ? AND deleted_at IS NULL", id, model.APICredentialStatusEnabled).
			Order("id ASC").First(&newDefault)
		s.db.Model(&model.APICredential{}).Where("id = ?", newDefault.ID).Update("is_default", true)
	}

	cred.Status = status
	cred.UpdatedAt = time.Now()

	return s.db.Save(cred).Error
}

// Delete 删除凭据（软删除）。
// 如果已绑定 Telegram 账号或为默认凭据，则禁止删除。
func (s *Service) Delete(id uint) error {
	cred, err := s.GetByID(id)
	if err != nil {
		return fmt.Errorf("凭据不存在")
	}

	// 检查是否是默认凭据
	if cred.IsDefault {
		return fmt.Errorf("不能删除默认凭据，请先切换默认凭据后再删除")
	}

	// 检查是否已绑定 Telegram 账号
	var count int64
	s.db.Model(&model.TelegramAccount{}).Where("api_credential_id = ?", id).Count(&count)
	if count > 0 {
		return fmt.Errorf("该凭据已绑定账号，只能禁用，不能删除")
	}

	return s.db.Delete(cred).Error
}

// GetAPIHash 获取解密后的 API Hash（仅供内部使用，不暴露到页面）。
func (s *Service) GetAPIHash(cred *model.APICredential) (string, error) {
	return security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
}

// CredentialSwitcherItem 是顶部栏切换器的项目。
type CredentialSwitcherItem struct {
	ID          uint
	DisplayName string
	APIIDMasked string
	APIHashHint string
	Label       string
	Selected    bool
}

// GetSwitcherItems 获取用于顶部栏切换的凭据列表。
func (s *Service) GetSwitcherItems(currentID uint) ([]CredentialSwitcherItem, error) {
	credentials, err := s.ListEnabled()
	if err != nil {
		return nil, err
	}

	items := make([]CredentialSwitcherItem, 0, len(credentials))
	for _, cred := range credentials {
		apiIDStr := fmt.Sprintf("%d", cred.APIID)
		apiIDMasked := apiIDStr
		if len(apiIDStr) > 4 {
			apiIDMasked = "****" + apiIDStr[len(apiIDStr)-4:]
		}

		label := fmt.Sprintf("%s · ID %s · Hash %s", cred.DisplayName, apiIDMasked, cred.APIHashHint)

		items = append(items, CredentialSwitcherItem{
			ID:          cred.ID,
			DisplayName: cred.DisplayName,
			APIIDMasked: apiIDMasked,
			APIHashHint: cred.APIHashHint,
			Label:       label,
			Selected:    cred.ID == currentID,
		})
	}

	return items, nil
}

// IsValidCredential 检查凭据是否有效（存在、启用、未删除）。
func (s *Service) IsValidCredential(id uint) bool {
	if id == 0 {
		return false
	}
	cred, err := s.GetByID(id)
	if err != nil {
		return false
	}
	return cred.Status == model.APICredentialStatusEnabled
}
