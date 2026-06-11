package tdlib_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..")
}

func TestTDLibAdapterReadmeExists(t *testing.T) {
	root := projectRoot()
	readmePath := filepath.Join(root, "internal", "telegramclient", "tdlib", "README.md")

	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("TDLib README.md 应存在")
	}
}

func TestTDLibReadmeMentionsClientAdapter(t *testing.T) {
	root := projectRoot()
	readmePath := filepath.Join(root, "internal", "telegramclient", "tdlib", "README.md")

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("读取 README.md 失败: %s", err)
	}

	s := string(content)
	if !strings.Contains(s, "ClientAdapter") {
		t.Error("TDLib README 应提及 ClientAdapter 接口")
	}
}

func TestTDLibReadmeMentionsNoTypeLeak(t *testing.T) {
	root := projectRoot()
	readmePath := filepath.Join(root, "internal", "telegramclient", "tdlib", "README.md")

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("读取 README.md 失败: %s", err)
	}

	s := string(content)
	// 应该提到类型不泄漏到业务层
	if !strings.Contains(s, "不得泄漏") && !strings.Contains(s, "不得泄露") && !strings.Contains(s, "不泄漏") {
		t.Error("TDLib README 应说明 TDLib 类型不得泄漏到业务层")
	}
}
