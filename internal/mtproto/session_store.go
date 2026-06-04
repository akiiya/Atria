package mtproto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/security"
)

// SessionFileInfo 表示 Session 文件信息。
type SessionFileInfo struct {
	Path        string
	Fingerprint string
	Size        int64
}

// SessionStore 定义 Session 文件存储接口。
type SessionStore interface {
	Save(accountID uint, data []byte) (SessionFileInfo, error)
	Load(path string) ([]byte, error)
	Delete(path string) error
	Exists(path string) bool
}

// FileSessionStore 是基于文件系统的 SessionStore 实现。
type FileSessionStore struct {
	dir string
	key []byte
}

// NewFileSessionStore 创建文件 SessionStore。
func NewFileSessionStore(dir string, key []byte) *FileSessionStore {
	return &FileSessionStore{dir: dir, key: key}
}

// Save 加密并保存 Session 数据到文件。
// 文件名使用 account_id，不包含手机号。
func (s *FileSessionStore) Save(accountID uint, data []byte) (SessionFileInfo, error) {
	// 确保目录存在
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return SessionFileInfo{}, fmt.Errorf("创建 Session 目录失败: %w", err)
	}

	// 生成文件名（不包含手机号）
	filename := fmt.Sprintf("session_%d.enc", accountID)
	path := filepath.Join(s.dir, filename)

	// 路径穿越检查
	if !s.isSafePath(path) {
		return SessionFileInfo{}, fmt.Errorf("不安全的文件路径")
	}

	// 加密数据
	encrypted, err := security.EncryptSessionData(s.key, data)
	if err != nil {
		return SessionFileInfo{}, fmt.Errorf("加密 Session 数据失败: %w", err)
	}

	// 写入临时文件后 rename，避免半写入
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, encrypted, 0600); err != nil {
		return SessionFileInfo{}, fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return SessionFileInfo{}, fmt.Errorf("重命名文件失败: %w", err)
	}

	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return SessionFileInfo{}, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 计算指纹
	fingerprint := crypto.Fingerprint(string(data))

	return SessionFileInfo{
		Path:        path,
		Fingerprint: fingerprint,
		Size:        info.Size(),
	}, nil
}

// Load 从文件加载并解密 Session 数据。
func (s *FileSessionStore) Load(path string) ([]byte, error) {
	// 路径穿越检查
	if !s.isSafePath(path) {
		return nil, fmt.Errorf("不安全的文件路径")
	}

	encrypted, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Session 文件失败: %w", err)
	}

	data, err := security.DecryptSessionData(s.key, encrypted)
	if err != nil {
		return nil, fmt.Errorf("解密 Session 数据失败: %w", err)
	}

	return data, nil
}

// Delete 删除 Session 文件。
func (s *FileSessionStore) Delete(path string) error {
	// 路径穿越检查
	if !s.isSafePath(path) {
		return fmt.Errorf("不安全的文件路径")
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 Session 文件失败: %w", err)
	}

	return nil
}

// Exists 检查 Session 文件是否存在。
func (s *FileSessionStore) Exists(path string) bool {
	if !s.isSafePath(path) {
		return false
	}

	_, err := os.Stat(path)
	return err == nil
}

// isSafePath 检查路径是否安全（防止路径穿越）。
func (s *FileSessionStore) isSafePath(path string) bool {
	// 清理路径
	cleaned := filepath.Clean(path)

	// 检查是否包含 ..
	if strings.Contains(cleaned, "..") {
		return false
	}

	// 检查是否在允许的目录下
	absDir, err := filepath.Abs(s.dir)
	if err != nil {
		return false
	}

	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return false
	}

	return strings.HasPrefix(absPath, absDir)
}
