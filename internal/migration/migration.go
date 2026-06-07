// Package migration 提供版本化数据迁移能力。
//
// 迁移在程序启动时自动执行，使用 Go 函数编写，不依赖 SQL 文件。
// 迁移版本记录在 data_migrations 表中，保证幂等执行。
package migration

import (
	"fmt"
	"log/slog"
	"sort"

	"gorm.io/gorm"
)

// Migration 表示一个数据迁移。
type Migration struct {
	Version     int                                 // 唯一版本号，递增
	Name        string                              // 迁移名称
	Description string                              // 迁移描述
	Run         func(db *gorm.DB, key []byte) error // 迁移执行函数
}

// dataMigrationRecord 是数据库中的迁移记录。
type dataMigrationRecord struct {
	Version int    `gorm:"primaryKey;not null"`
	Name    string `gorm:"size:256;not null"`
}

// TableName 返回表名。
func (dataMigrationRecord) TableName() string {
	return "data_migrations"
}

// registry 保存所有注册的迁移。
var registry []Migration

// Register 注册一个迁移。应在 init() 中调用。
func Register(m Migration) {
	registry = append(registry, m)
}

// Run 执行所有未执行的迁移。
// 在程序启动时调用，失败时返回 error 阻止启动。
func Run(db *gorm.DB, key []byte) error {
	// 确保迁移表存在
	if err := db.AutoMigrate(&dataMigrationRecord{}); err != nil {
		return fmt.Errorf("创建迁移版本表失败: %w", err)
	}

	// 获取已执行的迁移版本
	var applied []dataMigrationRecord
	if err := db.Find(&applied).Error; err != nil {
		return fmt.Errorf("查询已执行迁移失败: %w", err)
	}

	appliedSet := make(map[int]bool, len(applied))
	for _, r := range applied {
		appliedSet[r.Version] = true
	}

	// 按版本号排序（不修改全局 registry）
	sorted := make([]Migration, len(registry))
	copy(sorted, registry)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	// 按版本号排序执行
	executed := 0
	for _, m := range sorted {
		if appliedSet[m.Version] {
			continue
		}

		slog.Info("执行数据迁移",
			"version", m.Version,
			"name", m.Name,
			"description", m.Description,
		)

		if err := m.Run(db, key); err != nil {
			return fmt.Errorf("迁移 %d (%s) 失败: %w", m.Version, m.Name, err)
		}

		// 记录迁移已执行
		record := dataMigrationRecord{
			Version: m.Version,
			Name:    m.Name,
		}
		if err := db.Create(&record).Error; err != nil {
			return fmt.Errorf("记录迁移版本 %d 失败: %w", m.Version, err)
		}

		executed++
		slog.Info("数据迁移完成", "version", m.Version, "name", m.Name)
	}

	if executed > 0 {
		slog.Info("数据迁移全部完成", "executed", executed)
	} else {
		slog.Info("数据迁移：无需执行")
	}

	return nil
}

// GetAppliedVersions 返回已执行的迁移版本号列表（用于测试）。
func GetAppliedVersions(db *gorm.DB) ([]int, error) {
	var records []dataMigrationRecord
	if err := db.Find(&records).Error; err != nil {
		return nil, err
	}
	versions := make([]int, len(records))
	for i, r := range records {
		versions[i] = r.Version
	}
	return versions, nil
}

// Reset 清空迁移注册表（用于测试）。
func Reset() {
	registry = nil
}
