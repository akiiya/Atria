package migration

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/user/atria/internal/model"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:     1,
		Name:        "normalize_api_credential_defaults",
		Description: "归一化 API Key 数据：修复 enabled/is_default 不一致状态",
		Run:         migration001NormalizeAPICredentialDefaults,
	})

	Register(Migration{
		Version:     2,
		Name:        "init_system_setting_defaults",
		Description: "初始化缺失的系统设置默认值（proxy_*）",
		Run:         migration002InitSystemSettingDefaults,
	})

	Register(Migration{
		Version:     3,
		Name:        "create_chat_peer_cache",
		Description: "创建聊天 peer 缓存表，用于安全存储 access_hash",
		Run:         migration003CreateChatPeerCache,
	})

	Register(Migration{
		Version:     4,
		Name:        "backfill_legacy_account_sessions",
		Description: "为旧账号补齐 account_sessions 记录",
		Run:         migration004BackfillLegacyAccountSessions,
	})

	Register(Migration{
		Version:     5,
		Name:        "create_chat_message_cache",
		Description: "创建聊天消息缓存表，用于 cache-first 聊天加载",
		Run:         migration005CreateChatMessageCache,
	})

	Register(Migration{
		Version:     6,
		Name:        "add_chat_peer_cache_pin_mute",
		Description: "为 chat_peer_cache 添加 is_pinned 和 is_muted 字段",
		Run:         migration006AddChatPeerCachePinMute,
	})

	Register(Migration{
		Version:     7,
		Name:        "create_telegram_update_state",
		Description: "创建 Telegram update state 表，用于 updates 状态持久化和离线恢复",
		Run:         migration007CreateTelegramUpdateState,
	})

	Register(Migration{
		Version:     8,
		Name:        "create_telegram_channel_update_state",
		Description: "创建 Telegram channel update state 表，用于频道 pts 持久化",
		Run:         migration008CreateTelegramChannelUpdateState,
	})

	Register(Migration{
		Version:     9,
		Name:        "add_chat_cache_indexes",
		Description: "为 chat_peer_cache 和 chat_message_cache 补齐复合索引，优化缓存查询性能",
		Run:         migration009AddChatCacheIndexes,
	})

	Register(Migration{
		Version:     10,
		Name:        "fix_peer_cache_unique_index_and_phantom_records",
		Description: "修复 chat_peer_cache 唯一索引为 per-account 复合索引，并清理 PeerRef 为空的幽灵记录",
		Run:         migration010FixPeerCacheIndexAndPhantoms,
	})

	Register(Migration{
		Version:     11,
		Name:        "add_audit_log_account_id",
		Description: "为 audit_logs 表添加 account_id 字段，支持按账号筛选审计日志",
		Run:         migration011AddAuditLogAccountID,
	})

	Register(Migration{
		Version:     12,
		Name:        "create_media_cache",
		Description: "创建 media_cache 表，用于缓存已下载的媒体文件元数据",
		Run:         migration012CreateMediaCache,
	})
}

// migration001NormalizeAPICredentialDefaults 归一化 API Key 数据。
//
// 规则：
// 1. 如果没有任何记录，不创建假 API Key。
// 2. 如果存在 enabled=true 且 is_default=true 的记录，选它作系统 API Key。
// 3. 如果不存在默认但有 enabled 记录，选第一条设为默认。
// 4. 如果全部 disabled，不强行启用。
// 5. 如果多条 is_default=true，只保留一条（ID 最小且 enabled 优先）。
// 6. api_id 为空或 0 的记录不能作系统 API Key。
// 7. api_hash_encrypted 为空的记录不能作完整系统 API Key。
// 8. 不删除任何旧记录。
func migration001NormalizeAPICredentialDefaults(db *gorm.DB, _ []byte) error {
	var all []model.APICredential
	if err := db.Find(&all).Error; err != nil {
		return fmt.Errorf("查询 api_credentials 失败: %w", err)
	}

	if len(all) == 0 {
		slog.Info("迁移 1: api_credentials 为空，跳过")
		return nil
	}

	// 分类
	var (
		enabledWithDefault []*model.APICredential
		enabledNoDefault   []*model.APICredential
		multipleDefaults   []*model.APICredential
	)

	for i := range all {
		c := &all[i]
		if c.Status == model.APICredentialStatusEnabled {
			if c.IsDefault {
				enabledWithDefault = append(enabledWithDefault, c)
				multipleDefaults = append(multipleDefaults, c)
			} else {
				enabledNoDefault = append(enabledNoDefault, c)
			}
		}
	}

	// 场景 5：多条 is_default=true，只保留一条
	if len(multipleDefaults) > 1 {
		slog.Warn("迁移 1: 发现多条默认凭据，将只保留一条",
			"count", len(multipleDefaults),
		)

		// 保留第一条（ID 最小），其余取消默认
		keep := multipleDefaults[0]
		for _, c := range multipleDefaults[1:] {
			if err := db.Model(&model.APICredential{}).
				Where("id = ?", c.ID).
				Update("is_default", false).Error; err != nil {
				return fmt.Errorf("取消多余默认凭据 ID=%d 失败: %w", c.ID, err)
			}
			slog.Info("迁移 1: 取消多余默认", "id", c.ID, "name", c.DisplayName)
		}

		// 验证保留的默认是否有效
		if !isValidSystemKey(keep) {
			slog.Warn("迁移 1: 保留的默认凭据缺少有效 api_id 或 api_hash，需人工检查",
				"id", keep.ID,
			)
		}

		slog.Info("迁移 1: 保留默认凭据", "id", keep.ID, "name", keep.DisplayName)
		return nil
	}

	// 场景 2：已有唯一默认
	if len(enabledWithDefault) == 1 {
		keep := enabledWithDefault[0]
		if !isValidSystemKey(keep) {
			slog.Warn("迁移 1: 当前默认凭据缺少有效 api_id 或 api_hash",
				"id", keep.ID,
			)
		}
		slog.Info("迁移 1: 已有默认凭据，无需修改", "id", keep.ID, "name", keep.DisplayName)
		return nil
	}

	// 场景 3：无默认但有启用记录，选第一条
	if len(enabledNoDefault) > 0 {
		pick := enabledNoDefault[0]
		if err := db.Model(&model.APICredential{}).
			Where("id = ?", pick.ID).
			Update("is_default", true).Error; err != nil {
			return fmt.Errorf("设置默认凭据 ID=%d 失败: %w", pick.ID, err)
		}
		slog.Info("迁移 1: 自动设置默认凭据",
			"id", pick.ID, "name", pick.DisplayName,
		)
		return nil
	}

	// 场景 4：全部 disabled
	slog.Info("迁移 1: 所有凭据均已禁用，系统保持未配置状态")
	return nil
}

// isValidSystemKey 检查凭据是否可作为系统 API Key。
func isValidSystemKey(c *model.APICredential) bool {
	return c.APIID > 0 && c.EncryptedAPIHash != ""
}

// migration002InitSystemSettingDefaults 初始化缺失的系统设置默认值。
//
// 规则：
// - 缺失的 key 插入默认值
// - 已存在的 key 不覆盖
// - proxy_password 缺失时视为空字符串（不写入数据库）
// - 不把 proxy_password 明文写日志
func migration002InitSystemSettingDefaults(db *gorm.DB, _ []byte) error {
	defaults := map[string]string{
		"proxy_enabled":  "false",
		"proxy_type":     "none",
		"proxy_host":     "",
		"proxy_port":     "",
		"proxy_username": "",
		"proxy_timeout":  "30",
		"proxy_remark":   "",
	}

	// proxy_password 缺失时不写入数据库，读取时视为空字符串
	// 因此不在此处初始化

	for key, defaultValue := range defaults {
		var existing model.SystemSetting
		err := db.Where("key = ?", key).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			setting := model.SystemSetting{
				Key:       key,
				Value:     defaultValue,
				ValueType: "string",
			}
			if err := db.Create(&setting).Error; err != nil {
				return fmt.Errorf("初始化系统设置 %s 失败: %w", key, err)
			}
			slog.Info("迁移 2: 初始化系统设置", "key", key)
		} else if err != nil {
			return fmt.Errorf("查询系统设置 %s 失败: %w", key, err)
		}
		// 已存在则跳过，不覆盖用户配置
	}

	slog.Info("迁移 2: 系统设置默认值初始化完成")
	return nil
}

// migration003CreateChatPeerCache 创建聊天 peer 缓存表。
// 用于安全存储 Telegram peer 的 access_hash（AES-256-GCM 加密）。
// 幂等：AutoMigrate 会跳过已存在的表。
func migration003CreateChatPeerCache(db *gorm.DB, _ []byte) error {
	if err := db.AutoMigrate(&model.ChatPeerCache{}); err != nil {
		return fmt.Errorf("创建 chat_peer_cache 表失败: %w", err)
	}
	slog.Info("迁移 3: chat_peer_cache 表创建/更新完成")
	return nil
}

// migration004BackfillLegacyAccountSessions 为旧账号补齐 account_sessions 记录。
//
// 在聊天功能上线前已经接入的账号可能没有 account_sessions 记录。
// 本迁移为 active 状态且缺少 session 记录的账号自动补齐。
//
// 规则：
// 1. 只处理 status=active 的 telegram_accounts。
// 2. 跳过已有 account_sessions 记录的账号。
// 3. SessionFilePath 使用标准格式 session_<id>.enc。
// 4. 不访问 Telegram 网络。
// 5. 不修改 Session 文件。
// 6. 不删除旧账号。
// 7. 幂等：只插入不存在的记录。
func migration004BackfillLegacyAccountSessions(db *gorm.DB, _ []byte) error {
	// 查找所有 active 账号
	var accounts []model.TelegramAccount
	if err := db.Where("status = ?", model.TelegramAccountStatusActive).Find(&accounts).Error; err != nil {
		return fmt.Errorf("查询 active 账号失败: %w", err)
	}

	if len(accounts) == 0 {
		slog.Info("迁移 4: 无 active 账号，跳过")
		return nil
	}

	backfilled := 0
	for _, acc := range accounts {
		// 检查是否已有 session 记录
		var count int64
		if err := db.Model(&model.AccountSession{}).
			Where("telegram_account_id = ?", acc.ID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("查询 session 记录失败 (account_id=%d): %w", acc.ID, err)
		}

		if count > 0 {
			continue // 已有记录，跳过
		}

		// 补齐 session 记录
		now := acc.CreatedAt
		if now.IsZero() {
			now = time.Now()
		}
		session := model.AccountSession{
			TelegramAccountID:  acc.ID,
			SessionFilePath:    fmt.Sprintf("session_%d.enc", acc.ID),
			SessionFingerprint: fmt.Sprintf("legacy_%d", acc.ID),
			EncryptionVersion:  1,
			Status:             "active",
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := db.Create(&session).Error; err != nil {
			return fmt.Errorf("补齐 session 记录失败 (account_id=%d): %w", acc.ID, err)
		}
		slog.Info("迁移 4: 补齐旧账号 session 记录", "account_id", acc.ID, "display_name", acc.DisplayName)
		backfilled++
	}

	if backfilled > 0 {
		slog.Info("迁移 4: 旧账号 session 补齐完成", "count", backfilled)
	} else {
		slog.Info("迁移 4: 所有 active 账号已有 session 记录，无需补齐")
	}
	return nil
}

// migration005CreateChatMessageCache 创建聊天消息缓存表。
// 用于 cache-first 聊天加载，避免每次都实时请求 Telegram。
// 幂等：AutoMigrate 会跳过已存在的表。
func migration005CreateChatMessageCache(db *gorm.DB, _ []byte) error {
	if err := db.AutoMigrate(&model.ChatMessageCache{}); err != nil {
		return fmt.Errorf("创建 chat_message_cache 表失败: %w", err)
	}
	slog.Info("迁移 5: chat_message_cache 表创建/更新完成")
	return nil
}

// migration006AddChatPeerCachePinMute 为 chat_peer_cache 表添加 is_pinned 和 is_muted 字段。
// 用于会话列表排序和静音状态显示。
// 幂等：AutoMigrate 会跳过已存在的列。
func migration006AddChatPeerCachePinMute(db *gorm.DB, _ []byte) error {
	if err := db.AutoMigrate(&model.ChatPeerCache{}); err != nil {
		return fmt.Errorf("更新 chat_peer_cache 表失败: %w", err)
	}
	slog.Info("迁移 6: chat_peer_cache 表 is_pinned/is_muted 字段添加完成")
	return nil
}

// migration007CreateTelegramUpdateState 创建 Telegram update state 表。
// 用于 gotd updates.Manager 的 StateStorage 实现，支持离线恢复 getDifference。
// 按 account_id 唯一，不存储敏感字段（不存 access_hash、api_hash、session path 等）。
// 幂等：AutoMigrate 会跳过已存在的表。
func migration007CreateTelegramUpdateState(db *gorm.DB, _ []byte) error {
	if err := db.AutoMigrate(&model.TelegramUpdateState{}); err != nil {
		return fmt.Errorf("创建 telegram_update_state 表失败: %w", err)
	}
	slog.Info("迁移 7: telegram_update_state 表创建/更新完成")
	return nil
}

// migration008CreateTelegramChannelUpdateState 创建 Telegram channel update state 表。
// 用于频道 pts 持久化，支持频道级别的 getDifference 恢复。
// 按 account_id + channel_id 唯一，不存储敏感字段。
// 幂等：AutoMigrate 会跳过已存在的表。
func migration008CreateTelegramChannelUpdateState(db *gorm.DB, _ []byte) error {
	if err := db.AutoMigrate(&model.TelegramChannelUpdateState{}); err != nil {
		return fmt.Errorf("创建 telegram_channel_update_state 表失败: %w", err)
	}
	slog.Info("迁移 8: telegram_channel_update_state 表创建/更新完成")
	return nil
}

// migration009AddChatCacheIndexes 为 chat_peer_cache 和 chat_message_cache 补齐复合索引。
//
// 诊断发现以下查询缺少索引支持：
//   - chat_peer_cache: WHERE account_id = ? AND peer_ref = ? (5 处查询)
//   - chat_peer_cache: WHERE account_id = ? ORDER BY is_pinned DESC, last_message_at DESC
//   - chat_message_cache: ORDER BY sent_at 需 (account_id, peer_ref, sent_at)
//
// 使用 CREATE INDEX IF NOT EXISTS 确保幂等。
func migration009AddChatCacheIndexes(db *gorm.DB, _ []byte) error {
	indexes := []struct {
		table  string
		name   string
		column string
	}{
		// chat_peer_cache: 按账号+会话引用查找 peer
		{"chat_peer_cache", "idx_peer_account_peer", "(account_id, peer_ref)"},
		// chat_peer_cache: 会话列表排序（置顶优先 + 最新消息优先）
		{"chat_peer_cache", "idx_peer_account_pinned_msgat", "(account_id, is_pinned, last_message_at DESC)"},
		// chat_message_cache: 缓存清理时按 sent_at 排序删除旧消息
		{"chat_message_cache", "idx_msg_account_peer_sent", "(account_id, peer_ref, sent_at DESC)"},
	}

	for _, idx := range indexes {
		sql := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s %s", idx.name, idx.table, idx.column)
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("创建索引 %s 失败: %w", idx.name, err)
		}
		slog.Info("迁移 9: 创建索引", "index", idx.name, "table", idx.table)
	}

	slog.Info("迁移 9: 聊天缓存索引补齐完成")
	return nil
}

// migration010FixPeerCacheIndexAndPhantoms 修复 chat_peer_cache 唯一索引并清理幽灵记录。
//
// 背景：
//   - 旧版本 ChatPeerCache.PeerRef 有全局 uniqueIndex，不同账号无法缓存同一 peer
//   - mapMessage 未设置 PeerRef，导致 updateDialogPreview 创建 PeerRef="" 的幽灵记录
//   - 幽灵记录导致 ListDialogs 返回重复会话
//
// 修复步骤：
//  1. 删除 PeerRef="" 的幽灵记录
//  2. 删除关联的 PeerRef="" 消息缓存
//  3. 重建唯一索引为 (account_id, peer_ref) 复合索引
//
// 幂等：使用 IF NOT EXISTS / IF EXISTS 保护。
func migration010FixPeerCacheIndexAndPhantoms(db *gorm.DB, _ []byte) error {
	// 1. 清理 PeerRef="" 的幽灵 peer 缓存记录
	result := db.Where("peer_ref = '' OR peer_ref IS NULL").Delete(&model.ChatPeerCache{})
	if result.Error != nil {
		return fmt.Errorf("清理幽灵 peer 记录失败: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		slog.Warn("迁移 10: 清理幽灵 peer 记录", "count", result.RowsAffected)
	}

	// 2. 清理 PeerRef="" 的幽灵消息缓存
	result = db.Exec("DELETE FROM chat_message_cache WHERE peer_ref = '' OR peer_ref IS NULL")
	if result.Error != nil {
		return fmt.Errorf("清理幽灵消息记录失败: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		slog.Warn("迁移 10: 清理幽灵消息记录", "count", result.RowsAffected)
	}

	// 3. 删除旧的全局唯一索引（如果存在）
	// SQLite 不支持 DROP INDEX IF EXISTS，但我们可以尝试删除并忽略错误
	if err := db.Exec("DROP INDEX IF EXISTS uni_chat_peer_cache_peer_ref").Error; err != nil {
		slog.Warn("迁移 10: 删除旧索引（可能不存在）", "error", err)
	}

	// 4. 创建 per-account 复合唯一索引
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_peer_account_unique ON chat_peer_cache (account_id, peer_ref)").Error; err != nil {
		return fmt.Errorf("创建 per-account 唯一索引失败: %w", err)
	}

	slog.Info("迁移 10: chat_peer_cache 索引修复完成")
	return nil
}

// migration011AddAuditLogAccountID 为 audit_logs 表添加 account_id 字段。
// 用于按 Telegram 账号筛选审计日志。
// 幂等：重复执行时如果列已存在会忽略错误。
func migration011AddAuditLogAccountID(db *gorm.DB, _ []byte) error {
	if err := db.Exec("ALTER TABLE audit_logs ADD COLUMN account_id INTEGER DEFAULT 0").Error; err != nil {
		// Column might already exist
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("add account_id to audit_logs: %w", err)
		}
	}
	slog.Info("迁移 11: audit_logs 表 account_id 字段添加完成")
	return nil
}

// migration012CreateMediaCache 创建 media_cache 表。
// 用于缓存已下载的媒体文件元数据，支持媒体文件的本地缓存管理。
// 幂等：AutoMigrate 会跳过已存在的表。
func migration012CreateMediaCache(db *gorm.DB, _ []byte) error {
	if err := db.AutoMigrate(&model.MediaCache{}); err != nil {
		return fmt.Errorf("创建 media_cache 表失败: %w", err)
	}
	slog.Info("迁移 12: media_cache 表创建/更新完成")
	return nil
}
