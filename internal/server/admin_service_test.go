package server

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/auth"
	"github.com/user/atria/internal/model"

	"gorm.io/gorm"
)

// setupTestDB 创建测试用的内存数据库。
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&model.Admin{}); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}

	return db
}

func TestAdminService_IsInitialized_False(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	if svc.IsInitialized() {
		t.Error("新数据库应返回 IsInitialized=false")
	}
}

func TestAdminService_IsInitialized_True(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	if !svc.IsInitialized() {
		t.Error("初始化后应返回 IsInitialized=true")
	}
}

func TestAdminService_Initialize_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("第一次初始化失败: %s", err)
	}

	_, err = svc.Initialize(InitializeInput{Username: "admin2", Password: "password123456"})
	if err == nil {
		t.Error("重复初始化应该失败")
	}
}

func TestAdminService_Initialize_PasswordHashed(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	password := "password123456"
	admin, err := svc.Initialize(InitializeInput{Username: "admin", Password: password})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	if admin.PasswordHash == password {
		t.Error("password_hash 不应等于明文密码")
	}

	if admin.PasswordAlgo != "bcrypt" {
		t.Errorf("password_algo 应为 bcrypt，实际=%s", admin.PasswordAlgo)
	}

	if !auth.CheckPassword(password, admin.PasswordHash) {
		t.Error("密码哈希后应能通过校验")
	}
}

func TestAdminService_Login_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	admin, err := svc.Login("admin", "password123456")
	if err != nil {
		t.Fatalf("登录失败: %s", err)
	}

	if admin.Username != "admin" {
		t.Errorf("用户名不匹配，期望=admin，实际=%s", admin.Username)
	}

	if admin.LastLoginAt == nil {
		t.Error("LastLoginAt 应该被更新")
	}
}

func TestAdminService_Login_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	_, err = svc.Login("admin", "wrong_password")
	if err == nil {
		t.Error("错误密码登录应该失败")
	}

	// 错误提示不应泄露具体原因
	if err != nil && !strings.Contains(err.Error(), "用户名或密码不正确") {
		t.Errorf("错误提示应为通用消息，实际=%s", err.Error())
	}
}

func TestAdminService_Login_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	_, err = svc.Login("nonexistent", "password123456")
	if err == nil {
		t.Error("不存在用户名登录应该失败")
	}

	// 错误提示不应泄露用户是否存在
	if err != nil && !strings.Contains(err.Error(), "用户名或密码不正确") {
		t.Errorf("错误提示应为通用消息，实际=%s", err.Error())
	}
}

func TestAdminService_ChangePassword_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	admin, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	err = svc.ChangePassword(admin.ID, "password123456", "new_password_123456")
	if err != nil {
		t.Fatalf("修改密码失败: %s", err)
	}
}

func TestAdminService_ChangePassword_OldPasswordInvalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	admin, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	// 修改密码
	err = svc.ChangePassword(admin.ID, "password123456", "new_password_123456")
	if err != nil {
		t.Fatalf("修改密码失败: %s", err)
	}

	// 旧密码应失效
	_, err = svc.Login("admin", "password123456")
	if err == nil {
		t.Error("旧密码应该失效")
	}
}

func TestAdminService_ChangePassword_NewPasswordWorks(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	admin, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	// 修改密码
	err = svc.ChangePassword(admin.ID, "password123456", "new_password_123456")
	if err != nil {
		t.Fatalf("修改密码失败: %s", err)
	}

	// 新密码应可用
	_, err = svc.Login("admin", "new_password_123456")
	if err != nil {
		t.Fatalf("新密码登录失败: %s", err)
	}
}

func TestAdminService_ChangePassword_WrongCurrentPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	admin, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	err = svc.ChangePassword(admin.ID, "wrong_password", "new_password_123456")
	if err == nil {
		t.Error("错误当前密码应该失败")
	}
}

func TestAdminService_Initialize_EmptyUsername(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "", Password: "password123456"})
	if err == nil {
		t.Error("空用户名应该失败")
	}
}

func TestAdminService_Initialize_InvalidUsername(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "user@name", Password: "password123456"})
	if err == nil {
		t.Error("包含非法字符的用户名应该失败")
	}
}

func TestAdminService_Initialize_ShortPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	_, err := svc.Initialize(InitializeInput{Username: "admin", Password: "short"})
	if err == nil {
		t.Error("短密码应该失败")
	}
}

func TestAdminService_ChangePassword_MismatchConfirm(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdminService(db)

	admin, err := svc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化失败: %s", err)
	}

	// ChangePassword 不直接处理确认密码，但我们可以测试新密码校验
	err = svc.ChangePassword(admin.ID, "password123456", "short")
	if err == nil {
		t.Error("短新密码应该失败")
	}
}
