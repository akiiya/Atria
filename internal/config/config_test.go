package config

import (
	"testing"
	"time"
)

func TestLoad_DefaultValues(t *testing.T) {
	cfg := Load()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("期望 Host=127.0.0.1，实际=%s", cfg.Host)
	}
	if cfg.Port != "8080" {
		t.Errorf("期望 Port=8080，实际=%s", cfg.Port)
	}
	if cfg.DatabaseDriver != "sqlite" {
		t.Errorf("期望 DatabaseDriver=sqlite，实际=%s", cfg.DatabaseDriver)
	}
	if cfg.CookieName != "atria_session" {
		t.Errorf("期望 CookieName=atria_session，实际=%s", cfg.CookieName)
	}
	if cfg.CookieSameSite != "lax" {
		t.Errorf("期望 CookieSameSite=lax，实际=%s", cfg.CookieSameSite)
	}
	if !cfg.CSRFEnabled {
		t.Error("期望 CSRFEnabled=true")
	}
	if cfg.SessionTTL != 24*time.Hour {
		t.Errorf("期望 SessionTTL=24h，实际=%s", cfg.SessionTTL)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Errorf("默认配置应该通过校验，实际错误: %s", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := Load()
	cfg.Port = "99999"
	if err := cfg.Validate(); err == nil {
		t.Error("非法端口应该校验失败")
	}

	cfg.Port = "0"
	if err := cfg.Validate(); err == nil {
		t.Error("端口 0 应该校验失败")
	}

	cfg.Port = "abc"
	if err := cfg.Validate(); err == nil {
		t.Error("非数字端口应该校验失败")
	}
}

func TestValidate_InvalidDriver(t *testing.T) {
	cfg := Load()
	cfg.DatabaseDriver = "mongodb"
	if err := cfg.Validate(); err == nil {
		t.Error("非法数据库驱动应该校验失败")
	}
}

func TestValidate_EmptyHost(t *testing.T) {
	cfg := Load()
	cfg.Host = ""
	if err := cfg.Validate(); err == nil {
		t.Error("空 Host 应该校验失败")
	}
}

func TestValidate_InvalidSameSite(t *testing.T) {
	cfg := Load()
	cfg.CookieSameSite = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("非法 SameSite 应该校验失败")
	}
}

func TestValidate_EmptyCookieName(t *testing.T) {
	cfg := Load()
	cfg.CookieName = ""
	if err := cfg.Validate(); err == nil {
		t.Error("空 CookieName 应该校验失败")
	}
}

func TestValidate_EmptyCSRFHeader(t *testing.T) {
	cfg := Load()
	cfg.CSRFHeaderName = ""
	if err := cfg.Validate(); err == nil {
		t.Error("空 CSRFHeaderName 应该校验失败")
	}
}

func TestListenAddr(t *testing.T) {
	cfg := Load()
	addr := cfg.ListenAddr()
	if addr != "127.0.0.1:8080" {
		t.Errorf("期望 127.0.0.1:8080，实际=%s", addr)
	}
}

func TestMaskedDSN_SQLite(t *testing.T) {
	cfg := Load()
	dsn := cfg.MaskedDSN()
	if dsn != "./data/atria.db" {
		t.Errorf("SQLite DSN 不应脱敏，实际=%s", dsn)
	}
}
