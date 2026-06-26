package media

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── sanitizeLocalPath 测试 ──

func TestSanitizeLocalPath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"media/1/u_123/456/photo.jpg", filepath.FromSlash("media/1/u_123/456/photo.jpg")},
		{"../../../etc/passwd", ""},
		{"media/1/../../../etc/passwd", ""},
		{"/absolute/path", filepath.FromSlash("absolute/path")},
		{"", ""},
		// Windows 路径
		{"media\\1\\u_123\\photo.jpg", filepath.FromSlash("media/1/u_123/photo.jpg")},
		// 空字节注入（应被移除）
		{"media/1/photo\x00.jpg", filepath.FromSlash("media/1/photo.jpg")},
	}
	for _, tt := range tests {
		got := sanitizeLocalPath(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLocalPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── sanitizeFileName 测试 ──

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"photo.jpg", "photo.jpg"},
		{"../../../etc/passwd", "passwd"},
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
		{"path/to/file.txt", "file.txt"},
		{"path\\to\\file.txt", "file.txt"},
		{"normal-file.png", "normal-file.png"},
	}
	for _, tt := range tests {
		got := sanitizeFileName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── sanitizeFileName 不包含路径分隔符 ──

func TestSanitizeFileName_NoPathSeparators(t *testing.T) {
	inputs := []string{
		"../../../etc/passwd",
		"/absolute/path/file.txt",
		"path/to/file.txt",
		"path\\to\\file.txt",
		"..\\..\\windows\\system32\\config",
	}
	for _, input := range inputs {
		got := sanitizeFileName(input)
		if strings.Contains(got, "/") || strings.Contains(got, "\\") {
			t.Errorf("sanitizeFileName(%q) = %q contains path separator", input, got)
		}
	}
}

// ── isPathInsideDir 测试 ──

func TestIsPathInsideDir(t *testing.T) {
	tests := []struct {
		base, target string
		want         bool
	}{
		// 正常路径
		{"/data", "/data/media/1/photo.jpg", true},
		// 路径穿越
		{"/data", "/etc/passwd", false},
		// 精确匹配 dataDir 本身（不允许读取目录）
		{"/data", "/data", false},
		// 相似前缀绕过
		{"/data/media", "/data/media_evil/secret", false},
		// 上级目录
		{"/data/media", "/data/secret", false},
		// 深层嵌套
		{"/data", "/data/media/1/u_123/456/photo.jpg", true},
		// 空路径
		{"/data", "", false},
	}
	for _, tt := range tests {
		got := isPathInsideDir(tt.base, tt.target)
		if got != tt.want {
			t.Errorf("isPathInsideDir(%q, %q) = %v, want %v", tt.base, tt.target, got, tt.want)
		}
	}
}

// ── isPathInsideDir Windows 兼容 ──
// 仅在 Windows 上运行：Linux 的 filepath 不识别 drive letter，会将 C:\data 视为相对路径。

func TestIsPathInsideDir_WindowsPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("跳过：Windows 路径测试仅在 Windows 上运行")
	}
	tests := []struct {
		base, target string
		want         bool
	}{
		{"C:\\data", "C:\\data\\media\\photo.jpg", true},
		{"C:\\data", "C:\\secret\\file.txt", false},
		{"C:\\data", "C:\\data_evil\\secret", false},
	}
	for _, tt := range tests {
		got := isPathInsideDir(tt.base, tt.target)
		if got != tt.want {
			t.Errorf("isPathInsideDir(%q, %q) = %v, want %v", tt.base, tt.target, got, tt.want)
		}
	}
}

// ── lockKey 测试 ──

func TestLockKey(t *testing.T) {
	k1 := lockKey(1, "u_123", 456)
	k2 := lockKey(1, "u_123", 456)
	k3 := lockKey(2, "u_123", 456)
	k4 := lockKey(1, "u_456", 456)

	if k1 != k2 {
		t.Errorf("same inputs should produce same key")
	}
	if k1 == k3 {
		t.Errorf("different account_id should produce different key")
	}
	if k1 == k4 {
		t.Errorf("different peer_ref should produce different key")
	}
}
