package media

import (
	"path/filepath"
	"testing"
)

func TestSanitizeLocalPath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"media/1/u_123/456/photo.jpg", filepath.FromSlash("media/1/u_123/456/photo.jpg")},
		{"../../../etc/passwd", ""},
		{"media/1/../../../etc/passwd", ""},
		{"/absolute/path", filepath.FromSlash("absolute/path")},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeLocalPath(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLocalPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"photo.jpg", "photo.jpg"},
		{"../../../etc/passwd", "passwd"},
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
	}
	for _, tt := range tests {
		got := sanitizeFileName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
