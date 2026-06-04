package mtproto

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestStore(t *testing.T) (*FileSessionStore, string) {
	t.Helper()
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	store := NewFileSessionStore(dir, key)
	return store, dir
}

func TestSessionStore_SaveAndLoad(t *testing.T) {
	store, _ := setupTestStore(t)

	data := []byte("test session data")
	info, err := store.Save(1, data)
	if err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	if info.Path == "" {
		t.Error("文件路径不应为空")
	}

	loaded, err := store.Load(info.Path)
	if err != nil {
		t.Fatalf("加载失败: %s", err)
	}

	if !bytes.Equal(loaded, data) {
		t.Error("加载的数据应与保存的数据一致")
	}
}

func TestSessionStore_Save_NotPlaintext(t *testing.T) {
	store, _ := setupTestStore(t)

	data := []byte("test session data")
	info, err := store.Save(1, data)
	if err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	// 读取文件内容
	fileData, err := os.ReadFile(info.Path)
	if err != nil {
		t.Fatalf("读取文件失败: %s", err)
	}

	// 文件内容不应是明文
	if bytes.Equal(fileData, data) {
		t.Error("文件内容不应是明文")
	}
}

func TestSessionStore_Save_FilenameNoPhone(t *testing.T) {
	store, _ := setupTestStore(t)

	data := []byte("test session data")
	info, err := store.Save(42, data)
	if err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	filename := filepath.Base(info.Path)
	if strings.Contains(filename, "+") || strings.Contains(filename, "phone") {
		t.Errorf("文件名不应包含手机号信息: %s", filename)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store, _ := setupTestStore(t)

	data := []byte("test session data")
	info, err := store.Save(1, data)
	if err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	if !store.Exists(info.Path) {
		t.Error("文件应存在")
	}

	if err := store.Delete(info.Path); err != nil {
		t.Fatalf("删除失败: %s", err)
	}

	if store.Exists(info.Path) {
		t.Error("文件不应存在")
	}
}

func TestSessionStore_PathTraversalRejected(t *testing.T) {
	store, _ := setupTestStore(t)

	// 尝试路径穿越
	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		filepath.Join(store.dir, "..", "..", "secret.txt"),
	}

	for _, path := range maliciousPaths {
		_, err := store.Load(path)
		if err == nil {
			t.Errorf("路径穿越应被拒绝: %s", path)
		}

		err = store.Delete(path)
		if err == nil {
			t.Errorf("路径穿越应被拒绝: %s", path)
		}

		if store.Exists(path) {
			t.Errorf("路径穿越应返回 false: %s", path)
		}
	}
}

func TestSessionStore_WrongKeyCannotDecrypt(t *testing.T) {
	dir := t.TempDir()
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	store1 := NewFileSessionStore(dir, key1)
	data := []byte("secret session data")
	info, err := store1.Save(1, data)
	if err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	// 使用错误密钥加载
	store2 := NewFileSessionStore(dir, key2)
	_, err = store2.Load(info.Path)
	if err == nil {
		t.Error("错误密钥应无法解密")
	}
}

func TestSessionStore_FingerprintNotEmpty(t *testing.T) {
	store, _ := setupTestStore(t)

	data := []byte("test session data")
	info, err := store.Save(1, data)
	if err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	if info.Fingerprint == "" {
		t.Error("指纹不应为空")
	}
}

func TestSessionStore_FingerprintStable(t *testing.T) {
	store, _ := setupTestStore(t)

	data := []byte("test session data")
	info1, _ := store.Save(1, data)
	info2, _ := store.Save(1, data)

	if info1.Fingerprint != info2.Fingerprint {
		t.Error("相同数据的指纹应稳定")
	}
}
