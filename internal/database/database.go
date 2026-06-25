// Package database 提供数据库初始化和迁移功能。
package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init 初始化数据库连接。
func Init(driver, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch driver {
	case "sqlite":
		// 确保 SQLite 文件所在目录存在
		if dir := filepath.Dir(dsn); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return nil, fmt.Errorf("创建数据库目录失败: %w", err)
			}
		}
		dialector = sqlite.Open(dsn)
	case "postgres":
		// TODO: 实现 PostgreSQL 驱动
		// 需要: gorm.io/driver/postgres
		return nil, fmt.Errorf("postgres 驱动尚未实现")
	case "mysql", "mariadb":
		// TODO: 实现 MySQL/MariaDB 驱动
		// 需要: gorm.io/driver/mysql
		return nil, fmt.Errorf("%s 驱动尚未实现", driver)
	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	return db, nil
}

// AutoMigrate 执行数据库自动迁移。
//
// 迁移的模型：
//   - Admin: 管理员账号
//   - APICredential: API 凭据配置
//   - TelegramAccount: Telegram 账号
//   - AccountSession: Session 文件索引
//   - AccountSyncSnapshot: 账号同步快照
//   - AuditLog: 审计日志
//   - SystemSetting: 系统设置
//
// TODO: 正式版本可能需要版本化迁移（如 golang-migrate），而不是长期依赖 AutoMigrate。
func AutoMigrate(db *gorm.DB) error {
	slog.Info("执行数据库自动迁移")

	err := db.AutoMigrate(
		&model.Admin{},
		&model.APICredential{},
		&model.TelegramAccount{},
		&model.AccountSession{},
		&model.AccountSyncSnapshot{},
		&model.AuditLog{},
		&model.SystemSetting{},
		&model.ChatPeerCache{},
		&model.ChatMessageCache{},
		&model.MediaCache{},
		&model.TelegramUpdateState{},
		&model.TelegramChannelUpdateState{},
	)
	if err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	slog.Info("数据库迁移完成")
	return nil
}
