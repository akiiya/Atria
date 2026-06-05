package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/model"

	"gorm.io/gorm"
)

// AdminService 是管理员业务服务。
type AdminService struct {
	db *gorm.DB
}

// NewAdminService 创建管理员服务。
func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

// IsInitialized 检查系统是否已初始化（是否存在管理员）。
func (s *AdminService) IsInitialized() bool {
	var count int64
	s.db.Model(&model.Admin{}).Count(&count)
	return count > 0
}

// GetAdminByUsername 根据用户名获取管理员。
func (s *AdminService) GetAdminByUsername(username string) (*model.Admin, error) {
	var admin model.Admin
	err := s.db.Where("username = ?", username).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// InitializeInput 是初始化管理员的输入。
type InitializeInput struct {
	Username       string
	Password       string
	APIDisplayName string // 可选
	APIID          string // 可选
	APIHash        string // 可选
}

// Initialize 创建管理员（首次初始化）。
func (s *AdminService) Initialize(input InitializeInput) (*model.Admin, error) {
	// 检查是否已初始化
	if s.IsInitialized() {
		return nil, fmt.Errorf("系统已初始化，不可重复执行")
	}

	// 校验用户名
	if err := auth.ValidateUsername(input.Username); err != nil {
		return nil, err
	}

	// 校验密码
	if err := auth.ValidatePassword(input.Password); err != nil {
		return nil, err
	}

	// 哈希密码
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("密码哈希失败: %w", err)
	}

	// 创建管理员
	now := time.Now()
	admin := &model.Admin{
		Username:     input.Username,
		PasswordHash: hash,
		PasswordAlgo: "bcrypt",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.db.Create(admin).Error; err != nil {
		return nil, fmt.Errorf("创建管理员失败: %w", err)
	}

	return admin, nil
}

// Login 验证管理员登录。
func (s *AdminService) Login(username, password string) (*model.Admin, error) {
	// 查询管理员
	admin, err := s.GetAdminByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户名或密码不正确")
		}
		return nil, fmt.Errorf("查询管理员失败: %w", err)
	}

	// 验证密码
	if !auth.CheckPassword(password, admin.PasswordHash) {
		return nil, fmt.Errorf("用户名或密码不正确")
	}

	// 更新最后登录时间
	now := time.Now()
	s.db.Model(admin).Update("last_login_at", now)
	admin.LastLoginAt = &now

	return admin, nil
}

// ChangePassword 修改管理员密码。
func (s *AdminService) ChangePassword(adminID uint, currentPassword, newPassword string) error {
	// 获取管理员
	var admin model.Admin
	if err := s.db.First(&admin, adminID).Error; err != nil {
		return fmt.Errorf("管理员不存在")
	}

	// 验证当前密码
	if !auth.CheckPassword(currentPassword, admin.PasswordHash) {
		return fmt.Errorf("当前密码不正确")
	}

	// 校验新密码
	if err := auth.ValidatePassword(newPassword); err != nil {
		return err
	}

	// 哈希新密码
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}

	// 更新密码
	now := time.Now()
	if err := s.db.Model(&admin).Updates(map[string]any{
		"password_hash": hash,
		"password_algo": "bcrypt",
		"updated_at":    now,
	}).Error; err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	return nil
}
