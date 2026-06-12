package telegramclient_test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// projectRoot 返回项目根目录。
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// goFilesInDir 返回目录下的所有 .go 文件路径。
func goFilesInDir(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// fileContainsImport 检查文件是否包含指定的 import 路径。
func filesWithExtensionsInDir(dir string, extensions ...string) ([]string, error) {
	allowed := map[string]bool{}
	for _, ext := range extensions {
		allowed[ext] = true
	}

	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if allowed[filepath.Ext(path)] {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func fileContainsImport(filePath, importPath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	inImportBlock := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "import (" {
			inImportBlock = true
			continue
		}
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock && strings.Contains(line, importPath) {
			return true, nil
		}

		// 单行 import
		if strings.HasPrefix(line, "import ") && strings.Contains(line, importPath) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// TestNoGotdImportsInInternalChat 验证 internal/chat 不直接依赖 gotd。
func TestNoGotdImportsInInternalChat(t *testing.T) {
	root := projectRoot()
	chatDir := filepath.Join(root, "internal", "chat")

	files, err := goFilesInDir(chatDir)
	if err != nil {
		t.Fatalf("扫描目录失败: %s", err)
	}

	gotdPackages := []string{
		"github.com/gotd/td/tg",
		"github.com/gotd/td/telegram",
		"github.com/gotd/td/tgerr",
		"github.com/gotd/td/session",
		"github.com/gotd/td/telegram/auth",
		"github.com/gotd/td/telegram/updates",
	}

	for _, file := range files {
		for _, pkg := range gotdPackages {
			found, err := fileContainsImport(file, pkg)
			if err != nil {
				t.Errorf("读取文件 %s 失败: %s", file, err)
				continue
			}
			if found {
				t.Errorf("internal/chat 文件 %s 不应依赖 %s", filepath.Base(file), pkg)
			}
		}
	}
}

// TestNoGotdImportsInInternalServerChatHandlers 验证 server chat handler 不直接依赖 gotd tg/tgerr。
func TestNoGotdImportsInInternalServerChatHandlers(t *testing.T) {
	root := projectRoot()
	serverDir := filepath.Join(root, "internal", "server")

	chatFiles := []string{
		filepath.Join(serverDir, "chat_handler.go"),
		filepath.Join(serverDir, "api_handler.go"),
	}

	gotdPackages := []string{
		"github.com/user/atria/internal/telegramclient/gotd",
		"github.com/gotd/td/tg",
		"github.com/gotd/td/tgerr",
	}

	for _, file := range chatFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue
		}
		for _, pkg := range gotdPackages {
			found, err := fileContainsImport(file, pkg)
			if err != nil {
				t.Errorf("读取文件 %s 失败: %s", file, err)
				continue
			}
			if found {
				t.Errorf("server chat handler %s 不应依赖 %s", filepath.Base(file), pkg)
			}
		}
	}
}

// TestNoGotdTypesInTelegramClientTypes 验证 telegramclient types.go 不包含 gotd 类型引用。
func TestNoGotdTypesInTelegramClientTypes(t *testing.T) {
	root := projectRoot()
	typesFile := filepath.Join(root, "internal", "telegramclient", "types.go")

	content, err := os.ReadFile(typesFile)
	if err != nil {
		t.Fatalf("读取 types.go 失败: %s", err)
	}

	s := string(content)
	// 检查是否包含 gotd 类型引用（import 或类型使用），注释中的文档引用除外
	gotdTypeMarkers := []string{
		"github.com/gotd",
		"tg.",
		"telegram.Client",
	}
	for _, marker := range gotdTypeMarkers {
		if strings.Contains(s, marker) {
			t.Errorf("telegramclient/types.go 不应包含 gotd 类型引用 %q", marker)
		}
	}
}

// TestGotdImportsOnlyInAllowedPackages 验证 gotd import 只在允许的包中出现。
func TestGotdImportsOnlyInAllowedPackages(t *testing.T) {
	root := projectRoot()

	// 允许 gotd import 的目录
	allowedDirs := map[string]bool{
		filepath.Join(root, "internal", "telegramclient", "gotd"): true,
		filepath.Join(root, "internal", "mtproto"):                true,
		filepath.Join(root, "internal", "server"):                 true, // proxy_helper.go 使用 dcs.DialFunc
	}

	gotdPackages := []string{
		"github.com/gotd/td/tg",
		"github.com/gotd/td/telegram",
		"github.com/gotd/td/tgerr",
		"github.com/gotd/td/session",
		"github.com/gotd/td/telegram/auth",
		"github.com/gotd/td/telegram/updates",
		"github.com/gotd/td/telegram/dcs",
	}

	// 扫描所有 internal 目录下的 .go 文件
	internalDir := filepath.Join(root, "internal")
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// 检查是否在允许的目录中
		dir := filepath.Dir(path)
		if allowedDirs[dir] {
			return nil
		}

		for _, pkg := range gotdPackages {
			found, err := fileContainsImport(path, pkg)
			if err != nil {
				return nil
			}
			if found {
				t.Errorf("不允许的文件 %s 包含 gotd import %s", path, pkg)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描目录失败: %s", err)
	}
}

// TestChatServiceDependsOnClientAdapter 验证 ChatService 依赖 ClientAdapter 接口。
func TestChatServiceDependsOnClientAdapter(t *testing.T) {
	root := projectRoot()
	serviceFile := filepath.Join(root, "internal", "chat", "service.go")

	content, err := os.ReadFile(serviceFile)
	if err != nil {
		t.Fatalf("读取 service.go 失败: %s", err)
	}

	s := string(content)
	if !strings.Contains(s, "telegramclient.ClientAdapter") {
		t.Error("ChatService 应依赖 telegramclient.ClientAdapter")
	}
	if strings.Contains(s, "gotd/td/tg") {
		t.Error("ChatService 不应直接依赖 gotd/td/tg")
	}
}

// TestChatServiceDoesNotConstructTGRequests 验证 ChatService 不直接构造 gotd 请求。
func TestChatServiceDoesNotConstructTGRequests(t *testing.T) {
	root := projectRoot()
	serviceFile := filepath.Join(root, "internal", "chat", "service.go")

	content, err := os.ReadFile(serviceFile)
	if err != nil {
		t.Fatalf("读取 service.go 失败: %s", err)
	}

	s := string(content)
	gotdConstructs := []string{
		"MessagesGetHistoryRequest",
		"MessagesSendMessageRequest",
		"MessagesGetDialogsRequest",
		"InputPeerUser{",
		"InputPeerChannel{",
		"InputPeerChat{",
	}
	for _, c := range gotdConstructs {
		if strings.Contains(s, c) {
			t.Errorf("ChatService 不应包含 gotd 构造 %q", c)
		}
	}
}

// TestRuntimeTypesDoNotImportGotd 验证 telegramclient/runtime.go 不包含 gotd 引用。
func TestRuntimeTypesDoNotImportGotd(t *testing.T) {
	root := projectRoot()
	runtimeFile := filepath.Join(root, "internal", "telegramclient", "runtime.go")

	content, err := os.ReadFile(runtimeFile)
	if err != nil {
		t.Fatalf("读取 runtime.go 失败: %s", err)
	}

	s := string(content)
	gotdMarkers := []string{
		"github.com/gotd",
		"tg.",
		"telegram.",
	}
	for _, marker := range gotdMarkers {
		if strings.Contains(s, marker) {
			t.Errorf("telegramclient/runtime.go 不应包含 gotd 引用 %q", marker)
		}
	}
}

// TestEventBusTypesDoNotImportGotd 验证 telegramclient/event_bus.go 不包含 gotd 引用。
func TestEventBusTypesDoNotImportGotd(t *testing.T) {
	root := projectRoot()
	busFile := filepath.Join(root, "internal", "telegramclient", "event_bus.go")

	content, err := os.ReadFile(busFile)
	if err != nil {
		t.Fatalf("读取 event_bus.go 失败: %s", err)
	}

	s := string(content)
	gotdMarkers := []string{
		"github.com/gotd",
		"tg.",
	}
	for _, marker := range gotdMarkers {
		if strings.Contains(s, marker) {
			t.Errorf("telegramclient/event_bus.go 不应包含 gotd 引用 %q", marker)
		}
	}
}

// TestGotdRuntimeImportsOnlyInAllowedPackages 验证 gotd runtime 相关 import 只在允许的包中。
func TestGotdRuntimeImportsOnlyInAllowedPackages(t *testing.T) {
	root := projectRoot()

	// 允许 gotd/telegram/updates import 的目录
	allowedDirs := map[string]bool{
		filepath.Join(root, "internal", "telegramclient", "gotd"): true,
		filepath.Join(root, "internal", "mtproto"):                true,
	}

	updatesImport := "github.com/gotd/td/telegram/updates"

	internalDir := filepath.Join(root, "internal")
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		dir := filepath.Dir(path)
		if allowedDirs[dir] {
			return nil
		}

		found, err := fileContainsImport(path, updatesImport)
		if err != nil {
			return nil
		}
		if found {
			t.Errorf("不允许的文件 %s 包含 gotd/telegram/updates import", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描目录失败: %s", err)
	}
}

// TestTDLibReadmeMentionsRuntimeReplacement 验证 TDLib README 提及 runtime 替换。
func TestTDLibReadmeMentionsRuntimeReplacement(t *testing.T) {
	root := projectRoot()
	readmePath := filepath.Join(root, "internal", "telegramclient", "tdlib", "README.md")

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("读取 README.md 失败: %s", err)
	}

	s := string(content)
	if !strings.Contains(s, "RuntimeManager") {
		t.Error("TDLib README 应提及 RuntimeManager")
	}
}

// TestRealtimeServerDoesNotImportGotd 验证 realtime_ws.go 不依赖 gotd。
func TestRealtimeServerDoesNotImportGotd(t *testing.T) {
	root := projectRoot()
	wsFile := filepath.Join(root, "internal", "server", "realtime_ws.go")

	content, err := os.ReadFile(wsFile)
	if err != nil {
		t.Fatalf("读取 realtime_ws.go 失败: %s", err)
	}

	s := string(content)
	gotdMarkers := []string{
		"github.com/gotd",
		"tg.",
	}
	for _, marker := range gotdMarkers {
		if strings.Contains(s, marker) {
			t.Errorf("realtime_ws.go 不应包含 gotd 引用 %q", marker)
		}
	}
}

// TestFrontendTypesDoNotContainGotdNaming 验证前端类型不包含 gotd 命名。
func TestFrontendTypesDoNotContainGotdNaming(t *testing.T) {
	root := projectRoot()
	typesDir := filepath.Join(root, "frontend", "src", "types")

	files, err := filesWithExtensionsInDir(typesDir, ".ts", ".tsx", ".vue")
	if err != nil {
		t.Fatalf("扫描目录失败: %s", err)
	}
	if len(files) == 0 {
		t.Fatal("frontend/src/types must contain scanned type files")
	}

	gotdNames := []string{"gotd", "tg_", "telegram.Client", "InputPeer"}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		s := string(content)
		for _, name := range gotdNames {
			if strings.Contains(s, name) {
				t.Errorf("前端类型文件 %s 不应包含 gotd 命名 %q", filepath.Base(file), name)
			}
		}
	}
}

// TestTDLibReadmeMentionsWebSocketNoChange 验证 TDLib README 提及 WebSocket 层不需要修改。
func TestTDLibReadmeMentionsWebSocketNoChange(t *testing.T) {
	root := projectRoot()
	readmePath := filepath.Join(root, "internal", "telegramclient", "tdlib", "README.md")

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("读取 README.md 失败: %s", err)
	}

	s := string(content)
	if !strings.Contains(s, "WebSocket") && !strings.Contains(s, "websocket") {
		t.Fatal("TDLib README must mention that the WebSocket layer does not change")
	}
	if !strings.Contains(s, "UpdateEvent") {
		t.Fatal("TDLib README must mention publishing the same neutral UpdateEvent")
	}
}
