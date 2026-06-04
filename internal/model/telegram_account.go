package model

import "time"

// TelegramAccountStatus represents the status of a Telegram account.
type TelegramAccountStatus string

const (
	TelegramAccountStatusActive     TelegramAccountStatus = "active"
	TelegramAccountStatusBanned     TelegramAccountStatus = "banned"
	TelegramAccountStatusLoggedOut  TelegramAccountStatus = "logged_out"
	TelegramAccountStatusRestricted TelegramAccountStatus = "restricted"
)

// TelegramAccount represents a logged-in MTProto user account.
type TelegramAccount struct {
	ID               uint                  `gorm:"primaryKey" json:"id"`
	APICredentialID  uint                  `gorm:"index;not null" json:"api_credential_id"`
	UserID           int64                 `gorm:"index;not null" json:"user_id"`
	PhoneEncrypted   string                `gorm:"size:512;not null" json:"-"`       // Never expose in JSON
	PhoneFingerprint string                `gorm:"size:32" json:"phone_fingerprint"` // For masked display: last4
	Username         string                `gorm:"index;size:64" json:"username"`
	FirstName        string                `gorm:"size:128" json:"first_name"`
	LastName         string                `gorm:"size:128" json:"last_name"`
	DisplayName      string                `gorm:"size:256" json:"display_name"`
	Status           TelegramAccountStatus `gorm:"index;size:16;not null;default:active" json:"status"`
	IsPremium        bool                  `gorm:"not null;default:false" json:"is_premium"`
	IsRestricted     bool                  `gorm:"not null;default:false" json:"is_restricted"`
	IsScam           bool                  `gorm:"not null;default:false" json:"is_scam"`
	IsFake           bool                  `gorm:"not null;default:false" json:"is_fake"`
	LastSyncAt       *time.Time            `json:"last_sync_at"`
	CreatedAt        time.Time             `gorm:"not null" json:"created_at"`
	UpdatedAt        time.Time             `gorm:"not null" json:"updated_at"`

	// Relations
	APICredential APICredential         `gorm:"foreignKey:APICredentialID" json:"api_credential,omitempty"`
	Session       *AccountSession       `gorm:"foreignKey:TelegramAccountID" json:"session,omitempty"`
	Snapshots     []AccountSyncSnapshot `gorm:"foreignKey:TelegramAccountID" json:"snapshots,omitempty"`
}

// AccountSession represents the session file index for a Telegram account.
type AccountSession struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	TelegramAccountID  uint       `gorm:"uniqueIndex;not null" json:"telegram_account_id"`
	SessionFilePath    string     `gorm:"size:512;not null" json:"session_file_path"` // Relative path, no phone number
	SessionFingerprint string     `gorm:"size:64" json:"session_fingerprint"`
	EncryptionVersion  int        `gorm:"not null;default:1" json:"encryption_version"`
	Status             string     `gorm:"size:16;not null;default:active" json:"status"` // active, expired, invalid
	LastVerifiedAt     *time.Time `json:"last_verified_at"`
	CreatedAt          time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"not null" json:"updated_at"`
}

// AccountSyncSnapshotType represents the type of sync snapshot.
type AccountSyncSnapshotType string

const (
	SyncSnapshotTypeGroups   AccountSyncSnapshotType = "groups"
	SyncSnapshotTypeChannels AccountSyncSnapshotType = "channels"
	SyncSnapshotTypeProfile  AccountSyncSnapshotType = "profile"
)

// AccountSyncSnapshot represents a snapshot of account data from sync.
type AccountSyncSnapshot struct {
	ID                   uint                    `gorm:"primaryKey" json:"id"`
	TelegramAccountID    uint                    `gorm:"index;not null" json:"telegram_account_id"`
	SnapshotType         AccountSyncSnapshotType `gorm:"size:32;not null" json:"snapshot_type"`
	EncryptedPayloadPath string                  `gorm:"size:512" json:"-"`                // Path to encrypted payload file
	PayloadSummary       string                  `gorm:"size:1024" json:"payload_summary"` // Non-sensitive summary
	ItemCount            int                     `gorm:"not null;default:0" json:"item_count"`
	CreatedAt            time.Time               `gorm:"not null" json:"created_at"`
}
