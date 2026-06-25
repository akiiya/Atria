// Package config 提供 Atria 的配置加载和校验。
package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config 是 Atria 的完整配置结构。
type Config struct {
	// 应用名称
	AppName string

	// 服务监听地址
	Host string

	// 服务监听端口
	Port string

	// 数据根目录
	DataDir string

	// 数据库驱动：sqlite、postgres、mysql、mariadb
	DatabaseDriver string

	// 数据库连接字符串
	DatabaseDSN string

	// Session 文件存储目录
	SessionDir string

	// 日志目录
	LogDir string

	// 加密密钥（base64 编码的 32 字节密钥）
	SecretKey string

	// 密钥文件路径
	SecretKeyFile string

	// Cookie 名称
	CookieName string

	// Cookie 是否仅 HTTPS
	CookieSecure bool

	// Cookie SameSite 策略：lax、strict、none
	CookieSameSite string

	// 是否启用 CSRF 保护
	CSRFEnabled bool

	// CSRF Header 名称
	CSRFHeaderName string

	// CSRF 表单字段名称
	CSRFFieldName string

	// Web Session 有效期
	SessionTTL time.Duration

	// ===== 自更新配置 =====

	// UpdateEnabled 是否启用自更新
	UpdateEnabled bool

	// UpdateRepo GitHub 仓库，格式 owner/repo
	UpdateRepo string

	// UpdateCheckURL 自定义检查 URL（测试用）
	UpdateCheckURL string

	// UpdateDownloadBaseURL 自定义下载基础 URL（测试用）
	UpdateDownloadBaseURL string

	// UpdateDir 更新文件暂存目录
	UpdateDir string

	// UpdateBackupDir 旧二进制备份目录
	UpdateBackupDir string

	// UpdateTimeout HTTP 请求超时
	UpdateTimeout time.Duration

	// UpdateAllowPrerelease 是否允许预发布版本
	UpdateAllowPrerelease bool

	// UpdateRequireChecksum 是否要求校验 checksum
	UpdateRequireChecksum bool
}

// Load 从环境变量加载配置，未设置时使用默认值。
func Load() *Config {
	cfg := &Config{
		AppName:        "Atria",
		Host:           "127.0.0.1",
		Port:           "8080",
		DataDir:        "./data",
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "./data/atria.db",
		SessionDir:     "./data/sessions",
		LogDir:         "./data/logs",
		SecretKeyFile:  "./data/secret.key",
		CookieName:     "atria_session",
		CookieSecure:   false,
		CookieSameSite: "lax",
		CSRFEnabled:    true,
		CSRFHeaderName: "X-CSRF-Token",
		CSRFFieldName:  "csrf_token",
		SessionTTL:     24 * time.Hour,
	}

	if v := os.Getenv("ATRIA_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("ATRIA_PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("ATRIA_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("ATRIA_DB_DRIVER"); v != "" {
		cfg.DatabaseDriver = v
	}
	if v := os.Getenv("ATRIA_DB_DSN"); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := os.Getenv("ATRIA_SESSION_DIR"); v != "" {
		cfg.SessionDir = v
	}
	if v := os.Getenv("ATRIA_LOG_DIR"); v != "" {
		cfg.LogDir = v
	}
	if v := os.Getenv("ATRIA_SECRET_KEY"); v != "" {
		cfg.SecretKey = v
	}
	if v := os.Getenv("ATRIA_SECRET_KEY_FILE"); v != "" {
		cfg.SecretKeyFile = v
	}

	// 如果 SecretKeyFile 未通过环境变量设置，且 DataDir 已变化，
	// 则将 SecretKeyFile 更新为 DataDir 下的 secret.key
	if os.Getenv("ATRIA_SECRET_KEY_FILE") == "" && cfg.DataDir != "./data" {
		cfg.SecretKeyFile = filepath.Join(cfg.DataDir, "secret.key")
	}
	if v := os.Getenv("ATRIA_COOKIE_NAME"); v != "" {
		cfg.CookieName = v
	}
	if v := os.Getenv("ATRIA_COOKIE_SECURE"); v != "" {
		cfg.CookieSecure = v == "true" || v == "1"
	}
	if v := os.Getenv("ATRIA_COOKIE_SAMESITE"); v != "" {
		cfg.CookieSameSite = v
	}
	if v := os.Getenv("ATRIA_CSRF_ENABLED"); v != "" {
		cfg.CSRFEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ATRIA_SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SessionTTL = d
		}
	}

	// 自更新配置
	cfg.UpdateEnabled = true
	cfg.UpdateRepo = "akiiya/Atria"
	cfg.UpdateDir = filepath.Join(cfg.DataDir, "updates")
	cfg.UpdateBackupDir = filepath.Join(cfg.DataDir, "updates", "backups")
	cfg.UpdateTimeout = 60 * time.Second
	cfg.UpdateAllowPrerelease = true
	cfg.UpdateRequireChecksum = true

	if v := os.Getenv("ATRIA_UPDATE_ENABLED"); v != "" {
		cfg.UpdateEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ATRIA_UPDATE_REPO"); v != "" {
		cfg.UpdateRepo = v
	}
	if v := os.Getenv("ATRIA_UPDATE_CHECK_URL"); v != "" {
		cfg.UpdateCheckURL = v
	}
	if v := os.Getenv("ATRIA_UPDATE_DOWNLOAD_BASE_URL"); v != "" {
		cfg.UpdateDownloadBaseURL = v
	}
	if v := os.Getenv("ATRIA_UPDATE_DIR"); v != "" {
		cfg.UpdateDir = v
	}
	if v := os.Getenv("ATRIA_UPDATE_BACKUP_DIR"); v != "" {
		cfg.UpdateBackupDir = v
	}
	if v := os.Getenv("ATRIA_UPDATE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.UpdateTimeout = d
		}
	}
	if v := os.Getenv("ATRIA_UPDATE_ALLOW_PRERELEASE"); v != "" {
		cfg.UpdateAllowPrerelease = v == "true" || v == "1"
	}
	if v := os.Getenv("ATRIA_UPDATE_REQUIRE_CHECKSUM"); v != "" {
		cfg.UpdateRequireChecksum = v == "true" || v == "1"
	}

	return cfg
}

// allowedDrivers 是允许的数据库驱动列表。
var allowedDrivers = map[string]bool{
	"sqlite":   true,
	"postgres": true,
	"mysql":    true,
	"mariadb":  true,
}

// allowedSameSite 是允许的 Cookie SameSite 值。
var allowedSameSite = map[string]bool{
	"lax":    true,
	"strict": true,
	"none":   true,
}

// Validate 校验配置合法性。
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host 不能为空")
	}

	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port 必须在 1-65535 之间，当前值: %s", c.Port)
	}

	if c.DataDir == "" {
		return fmt.Errorf("data_dir 不能为空")
	}

	if c.SessionDir == "" {
		return fmt.Errorf("session_dir 不能为空")
	}

	if c.LogDir == "" {
		return fmt.Errorf("log_dir 不能为空")
	}

	if !allowedDrivers[c.DatabaseDriver] {
		return fmt.Errorf("database_driver 不合法，允许值: sqlite, postgres, mysql, mariadb，当前值: %s", c.DatabaseDriver)
	}

	if c.DatabaseDriver == "sqlite" && c.DatabaseDSN == "" {
		return fmt.Errorf("sqlite 模式下 database_dsn 不能为空")
	}

	if c.CookieName == "" {
		return fmt.Errorf("cookie_name 不能为空")
	}

	if !allowedSameSite[c.CookieSameSite] {
		return fmt.Errorf("cookie_same_site 不合法，允许值: lax, strict, none，当前值: %s", c.CookieSameSite)
	}

	if c.SessionTTL <= 0 {
		return fmt.Errorf("session_ttl 必须大于 0")
	}

	if c.CSRFHeaderName == "" {
		return fmt.Errorf("csrf_header_name 不能为空")
	}

	if c.CSRFFieldName == "" {
		return fmt.Errorf("csrf_field_name 不能为空")
	}

	return nil
}

// EnsureDirs 创建配置中需要的所有目录。
func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.DataDir,
		c.SessionDir,
		c.LogDir,
	}

	// 自更新目录
	if c.UpdateDir != "" {
		dirs = append(dirs, c.UpdateDir)
	}
	if c.UpdateBackupDir != "" {
		dirs = append(dirs, c.UpdateBackupDir)
	}

	// 确保密钥文件所在目录存在
	if c.SecretKeyFile != "" {
		keyDir := filepath.Dir(c.SecretKeyFile)
		if keyDir != "" && keyDir != "." {
			dirs = append(dirs, keyDir)
		}
	}

	// 去重
	seen := make(map[string]bool)
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("无法解析目录路径 %s: %w", dir, err)
		}
		if seen[absDir] {
			continue
		}
		seen[absDir] = true

		if err := os.MkdirAll(absDir, 0700); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	// SQLite 数据库文件目录
	if c.DatabaseDriver == "sqlite" && c.DatabaseDSN != "" {
		dbDir := filepath.Dir(c.DatabaseDSN)
		if dbDir != "" && dbDir != "." {
			absDbDir, _ := filepath.Abs(dbDir)
			if !seen[absDbDir] {
				if err := os.MkdirAll(dbDir, 0700); err != nil {
					return fmt.Errorf("创建数据库目录 %s 失败: %w", dbDir, err)
				}
			}
		}
	}

	return nil
}

// ListenAddr 返回完整的监听地址。
func (c *Config) ListenAddr() string {
	return c.Host + ":" + c.Port
}

// CookieSameSiteMode 返回 http.SameSite 常量，用于 Gin 的 SetSameSite。
func (c *Config) CookieSameSiteMode() http.SameSite {
	switch c.CookieSameSite {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// MaskedDSN 返回脱敏的数据库连接字符串（用于日志）。
func (c *Config) MaskedDSN() string {
	if c.DatabaseDriver == "sqlite" {
		return c.DatabaseDSN
	}
	// 非 SQLite 隐藏密码部分
	if idx := strings.Index(c.DatabaseDSN, "@"); idx > 0 {
		return "***@" + c.DatabaseDSN[idx+1:]
	}
	return "***"
}
