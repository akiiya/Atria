package auth

import (
	"testing"
)

func TestHashPassword_NotEqualToPlaintext(t *testing.T) {
	password := "my_secure_password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("哈希密码失败: %s", err)
	}

	if hash == password {
		t.Error("哈希值不应等于明文密码")
	}
}

func TestCheckPassword_Correct(t *testing.T) {
	password := "my_secure_password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("哈希密码失败: %s", err)
	}

	if !CheckPassword(password, hash) {
		t.Error("正确密码校验应该成功")
	}
}

func TestCheckPassword_Incorrect(t *testing.T) {
	password := "my_secure_password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("哈希密码失败: %s", err)
	}

	if CheckPassword("wrong_password", hash) {
		t.Error("错误密码校验应该失败")
	}
}

func TestValidatePassword_TooShort(t *testing.T) {
	err := ValidatePassword("short")
	if err == nil {
		t.Error("短密码应该校验失败")
	}
}

func TestValidatePassword_Empty(t *testing.T) {
	err := ValidatePassword("")
	if err == nil {
		t.Error("空密码应该校验失败")
	}
}

func TestValidatePassword_WhitespaceOnly(t *testing.T) {
	err := ValidatePassword("          ")
	if err == nil {
		t.Error("全空白密码应该校验失败")
	}
}

func TestValidatePassword_Valid(t *testing.T) {
	err := ValidatePassword("valid_password_123")
	if err != nil {
		t.Errorf("有效密码不应报错: %s", err)
	}
}

func TestValidateUsername_TooShort(t *testing.T) {
	err := ValidateUsername("ab")
	if err == nil {
		t.Error("短用户名应该校验失败")
	}
}

func TestValidateUsername_InvalidChars(t *testing.T) {
	err := ValidateUsername("user@name")
	if err == nil {
		t.Error("包含非法字符的用户名应该校验失败")
	}
}

func TestValidateUsername_Valid(t *testing.T) {
	err := ValidateUsername("admin_user-123")
	if err != nil {
		t.Errorf("有效用户名不应报错: %s", err)
	}
}
