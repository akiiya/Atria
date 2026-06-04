package mtproto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/atria/internal/security"
)

// GotdSessionStorage 实现 gotd/td 的 session.Storage 接口。
// 负责把 gotd session bytes 加密存储到 Atria 的文件系统。
// 支持 per-flow 隔离，每个 flow 使用独立的 session 文件。
type GotdSessionStorage struct {
	dir  string
	key  []byte
	name string // session 文件名（不含扩展名），如 "flow_abc123"
}

// NewGotdSessionStorage 创建 GotdSessionStorage。
// name 参数用于区分不同的 session 文件，如 "flow_abc123"。
func NewGotdSessionStorage(dir string, key []byte, name string) *GotdSessionStorage {
	return &GotdSessionStorage{dir: dir, key: key, name: name}
}

// sessionPath 返回 session 文件路径。
func (s *GotdSessionStorage) sessionPath() string {
	return filepath.Join(s.dir, s.name+".enc")
}

// LoadSession 加载并解密 Session 数据。
func (s *GotdSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	path := s.sessionPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 无 Session
		}
		return nil, fmt.Errorf("读取 Session 文件失败: %w", err)
	}

	decrypted, err := security.DecryptSessionData(s.key, data)
	if err != nil {
		return nil, fmt.Errorf("解密 Session 数据失败: %w", err)
	}

	return decrypted, nil
}

// StoreSession 加密并保存 Session 数据。
func (s *GotdSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	// 确保目录存在
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	path := s.sessionPath()

	encrypted, err := security.EncryptSessionData(s.key, data)
	if err != nil {
		return fmt.Errorf("加密 Session 数据失败: %w", err)
	}

	// 写入临时文件后 rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, encrypted, 0600); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	return nil
}

// ExportSession 读取并解密当前 session 数据。
func (s *GotdSessionStorage) ExportSession() ([]byte, error) {
	path := s.sessionPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 Session 文件失败: %w", err)
	}

	decrypted, err := security.DecryptSessionData(s.key, data)
	if err != nil {
		return nil, fmt.Errorf("解密 Session 数据失败: %w", err)
	}

	return decrypted, nil
}

// DeleteSession 删除 session 文件。
func (s *GotdSessionStorage) DeleteSession() error {
	path := s.sessionPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 Session 文件失败: %w", err)
	}
	return nil
}

// SessionExists 检查 session 文件是否存在。
func (s *GotdSessionStorage) SessionExists() bool {
	_, err := os.Stat(s.sessionPath())
	return err == nil
}

// FileBackedSessionStorage 实现 gotd/td 的 session.Storage 接口。
// 用于基于正式加密 session 文件的读写操作（如 SyncProfile、CheckSession）。
type FileBackedSessionStorage struct {
	key      []byte
	filePath string
}

// NewFileBackedSessionStorage 创建 FileBackedSessionStorage。
func NewFileBackedSessionStorage(key []byte, filePath string) *FileBackedSessionStorage {
	return &FileBackedSessionStorage{key: key, filePath: filePath}
}

// LoadSession 加载并解密 Session 数据。
func (s *FileBackedSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Session 文件不存在: %s", s.filePath)
		}
		return nil, fmt.Errorf("读取 Session 文件失败: %w", err)
	}

	decrypted, err := security.DecryptSessionData(s.key, data)
	if err != nil {
		return nil, fmt.Errorf("解密 Session 数据失败: %w", err)
	}

	return decrypted, nil
}

// StoreSession 加密并保存 Session 数据。
// 注意：对于正式 session 文件，更新操作需要谨慎。
func (s *FileBackedSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	encrypted, err := security.EncryptSessionData(s.key, data)
	if err != nil {
		return fmt.Errorf("加密 Session 数据失败: %w", err)
	}

	// 写入临时文件后 rename
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, encrypted, 0600); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	return nil
}
