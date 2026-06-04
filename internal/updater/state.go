package updater

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadState 从文件加载更新状态。
func LoadState(path string) (*UpdateState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UpdateState{Status: StatusIdle}, nil
		}
		return nil, fmt.Errorf("读取状态文件失败: %w", err)
	}

	var state UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("解析状态文件失败: %w", err)
	}

	return &state, nil
}

// SaveState 保存更新状态到文件。
func SaveState(path string, state UpdateState) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}

	// 写入临时文件后 rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	return nil
}
