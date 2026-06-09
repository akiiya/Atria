package migration

import (
	"fmt"
	"log/slog"

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
