package server

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/model"
	"gorm.io/gorm"
)

func setupProxyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}
	return db
}

func insertSetting(db *gorm.DB, key, value string) {
	db.Create(&model.SystemSetting{Key: key, Value: value, ValueType: "string"})
}

func TestBuildProxyDialer_ProxyPasswordMissing_NoRecordNotFoundNoise(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	// 配置代理但不创建 proxy_password 记录
	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "socks5")
	insertSetting(db, "proxy_host", "127.0.0.1")
	insertSetting(db, "proxy_port", "1080")
	// 故意不创建 proxy_password

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("proxy_password 缺失时不应报错: %s", err)
	}
	if dialer == nil {
		t.Fatal("proxy_password 缺失时应返回有效 dialer")
	}
}

func TestBuildProxyDialer_ProxyPasswordMissing_StillUsesProxy(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "socks5")
	insertSetting(db, "proxy_host", "127.0.0.1")
	insertSetting(db, "proxy_port", "1080")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("不应报错: %s", err)
	}
	if dialer == nil {
		t.Fatal("应返回有效 dialer，不应直连")
	}
}

func TestBuildProxyDialer_ProxyPasswordDecryptFailed_ReturnsProxyConfigInvalid(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "socks5")
	insertSetting(db, "proxy_host", "127.0.0.1")
	insertSetting(db, "proxy_port", "1080")
	// 插入无法解密的密码
	insertSetting(db, "proxy_password", "invalid_encrypted_data_that_cannot_be_decrypted")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err == nil {
		t.Fatal("proxy_password 解密失败时应返回错误")
	}
	if dialer != nil {
		t.Fatal("proxy_password 解密失败时不应返回 dialer")
	}
}

func TestBuildProxyDialer_ProxyDisabled_ReturnsNil(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "false")
	insertSetting(db, "proxy_type", "none")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("代理禁用时不应报错: %s", err)
	}
	if dialer != nil {
		t.Fatal("代理禁用时应返回 nil dialer")
	}
}

func TestBuildProxyDialer_ProxyTypeNone_ReturnsNil(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "none")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("proxy_type=none 时不应报错: %s", err)
	}
	if dialer != nil {
		t.Fatal("proxy_type=none 时应返回 nil dialer")
	}
}

func TestBuildProxyDialer_NoSettings_ReturnsNil(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	// 不插入任何设置
	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("无设置时不应报错: %s", err)
	}
	if dialer != nil {
		t.Fatal("无设置时应返回 nil dialer")
	}
}

func TestBuildProxyDialer_HTTPSProxy_ReturnsDialer(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "https")
	insertSetting(db, "proxy_host", "proxy.example.com")
	insertSetting(db, "proxy_port", "8080")
	insertSetting(db, "proxy_username", "user")
	insertSetting(db, "proxy_timeout", "60")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("HTTPS 代理配置应正常: %s", err)
	}
	if dialer == nil {
		t.Fatal("HTTPS 代理应返回有效 dialer")
	}
}

func TestBuildProxyDialer_MissingHost_ReturnsError(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "socks5")
	// 缺少 host
	insertSetting(db, "proxy_port", "1080")

	_, err := BuildProxyDialerFromDB(db, key)
	if err == nil {
		t.Fatal("缺少 host 时应返回错误")
	}
}

func TestBuildProxyDialer_InvalidPort_ReturnsError(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "socks5")
	insertSetting(db, "proxy_host", "127.0.0.1")
	insertSetting(db, "proxy_port", "invalid")

	_, err := BuildProxyDialerFromDB(db, key)
	if err == nil {
		t.Fatal("无效端口时应返回错误")
	}
}

// ===== API Proxy 测试 =====

func TestBuildProxyDialer_APIProxy_ReturnsError(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "api_proxy")
	insertSetting(db, "api_proxy_url", "https://xxx.domain.com")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err == nil {
		t.Fatal("api_proxy 类型应返回错误（不适用于 MTProto）")
	}
	if dialer != nil {
		t.Fatal("api_proxy 类型不应返回 dialer")
	}
	// 错误信息应明确说明原因
	if !containsSubstr(err.Error(), "MTProto") {
		t.Errorf("错误信息应提及 MTProto，实际: %s", err.Error())
	}
}

func TestBuildProxyDialer_APIProxy_DoesNotCauseInfiniteConnecting(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "api_proxy")
	insertSetting(db, "api_proxy_url", "https://xxx.domain.com")

	// 多次调用应每次都返回明确错误，不会卡住
	for i := 0; i < 5; i++ {
		_, err := BuildProxyDialerFromDB(db, key)
		if err == nil {
			t.Fatalf("第 %d 次调用应返回错误", i+1)
		}
	}
}

func TestBuildProxyDialer_Socks5_StillWorks(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "socks5")
	insertSetting(db, "proxy_host", "127.0.0.1")
	insertSetting(db, "proxy_port", "1080")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("socks5 代理应正常: %s", err)
	}
	if dialer == nil {
		t.Fatal("socks5 代理应返回有效 dialer")
	}
}

func TestBuildProxyDialer_HTTPS_StillWorks(t *testing.T) {
	db := setupProxyTestDB(t)
	key := make([]byte, 32)

	insertSetting(db, "proxy_enabled", "true")
	insertSetting(db, "proxy_type", "https")
	insertSetting(db, "proxy_host", "proxy.example.com")
	insertSetting(db, "proxy_port", "443")

	dialer, err := BuildProxyDialerFromDB(db, key)
	if err != nil {
		t.Fatalf("https 代理应正常: %s", err)
	}
	if dialer == nil {
		t.Fatal("https 代理应返回有效 dialer")
	}
}

// ===== API Proxy URL 校验测试 =====

func TestValidateAPIProxyURL_ValidHTTPS(t *testing.T) {
	err := validateAPIProxyURL("https://xxx.domain.com")
	if err != nil {
		t.Errorf("有效 HTTPS URL 不应报错: %s", err)
	}
}

func TestValidateAPIProxyURL_RejectsHTTP(t *testing.T) {
	err := validateAPIProxyURL("http://xxx.domain.com")
	if err == nil {
		t.Fatal("http:// 应被拒绝")
	}
}

func TestValidateAPIProxyURL_RejectsEmptyHost(t *testing.T) {
	err := validateAPIProxyURL("https://")
	if err == nil {
		t.Fatal("空主机应被拒绝")
	}
}

func TestValidateAPIProxyURL_RejectsQuery(t *testing.T) {
	err := validateAPIProxyURL("https://xxx.domain.com?token=abc")
	if err == nil {
		t.Fatal("包含 query 的 URL 应被拒绝")
	}
}

func TestValidateAPIProxyURL_RejectsFragment(t *testing.T) {
	err := validateAPIProxyURL("https://xxx.domain.com/#abc")
	if err == nil {
		t.Fatal("包含 fragment 的 URL 应被拒绝")
	}
}

func TestValidateAPIProxyURL_RejectsFTP(t *testing.T) {
	err := validateAPIProxyURL("ftp://xxx.domain.com")
	if err == nil {
		t.Fatal("ftp:// 应被拒绝")
	}
}

func TestValidateAPIProxyURL_RejectsTooLong(t *testing.T) {
	longURL := "https://example.com/" + string(make([]byte, 2030))
	err := validateAPIProxyURL(longURL)
	if err == nil {
		t.Fatal("超长 URL 应被拒绝")
	}
}

func TestValidateAPIProxyURL_AllowsWithPath(t *testing.T) {
	err := validateAPIProxyURL("https://xxx.domain.com/api/v1")
	if err != nil {
		t.Errorf("带路径的 URL 不应报错: %s", err)
	}
}

func TestNormalizeAPIProxyURL_TrimsSlash(t *testing.T) {
	result := normalizeAPIProxyURL("https://xxx.domain.com/")
	if result != "https://xxx.domain.com" {
		t.Errorf("应去掉末尾 slash，实际: %s", result)
	}
}

func TestNormalizeAPIProxyURL_TrimsSpace(t *testing.T) {
	result := normalizeAPIProxyURL("  https://xxx.domain.com  ")
	if result != "https://xxx.domain.com" {
		t.Errorf("应去掉首尾空格，实际: %s", result)
	}
}

func TestNormalizeAPIProxyURL_PreservesPath(t *testing.T) {
	result := normalizeAPIProxyURL("https://xxx.domain.com/api/v1/")
	if result != "https://xxx.domain.com/api/v1" {
		t.Errorf("应保留路径但去掉末尾 slash，实际: %s", result)
	}
}

// containsSubstr 检查字符串是否包含子串。
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
