package security

import (
	"strings"
	"testing"
)

func TestSanitizeErrorMessage_RemovesFilePaths(t *testing.T) {
	input := "session file /data/sessions/test.session not found"
	out := SanitizeErrorMessage(input)
	if strings.Contains(out, "/data/sessions/test.session") {
		t.Errorf("应脱敏文件路径，实际: %s", out)
	}
}

func TestSanitizeErrorMessage_RemovesHexStrings(t *testing.T) {
	input := "api_hash abcdef0123456789abcdef0123456789ab is invalid"
	out := SanitizeErrorMessage(input)
	if strings.Contains(out, "abcdef0123456789abcdef0123456789ab") {
		t.Errorf("应脱敏长十六进制串，实际: %s", out)
	}
}

func TestSanitizeErrorMessage_RemovesPhoneNumbers(t *testing.T) {
	input := "phone +8613800138000 already registered"
	out := SanitizeErrorMessage(input)
	if strings.Contains(out, "+8613800138000") {
		t.Errorf("应脱敏手机号，实际: %s", out)
	}
}

func TestSanitizeErrorMessage_RemovesSensitiveFragments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		bad   string
	}{
		{"api_hash", "error: api_hash:deadbeef", "deadbeef"},
		{"proxy_password", "error: proxy_password:mysecret", "mysecret"},
		{"session_path", "error: session_path:/tmp/test.session", "/tmp/test.session"},
		{"access_hash", "error: access_hash:123456", "123456"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := SanitizeErrorMessage(tt.input)
			if strings.Contains(out, tt.bad) {
				t.Errorf("应脱敏 %q，实际: %s", tt.bad, out)
			}
		})
	}
}

func TestSanitizeErrorMessage_PreservesSafeMessages(t *testing.T) {
	input := "connection refused: FLOOD_WAIT"
	out := SanitizeErrorMessage(input)
	if out != input {
		t.Errorf("安全消息应保留原样，实际: %s", out)
	}
}
