# Atria — Data Model

## Overview

Atria uses GORM for database abstraction. The default database is SQLite, with PostgreSQL and MySQL/MariaDB support via driver switching.

All sensitive fields are encrypted at the application layer before storage. The database never contains plaintext API hashes, phone numbers, or session data.

## Tables

### admins

**Purpose:** Single administrator account. Only one admin exists in the system.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uint | PK, auto-increment | Primary key |
| username | string(64) | UNIQUE, NOT NULL, indexed | Admin username |
| password_hash | string(256) | NOT NULL | Bcrypt/argon2id hash. **Never exposed in API responses.** |
| password_algo | string(32) | NOT NULL | Hash algorithm identifier (e.g., "bcrypt", "argon2id") |
| is_initialized | bool | NOT NULL, default false | Set to true after first admin setup |
| last_login_at | datetime | nullable | Last successful login timestamp |
| created_at | datetime | NOT NULL | Record creation time |
| updated_at | datetime | NOT NULL | Last modification time |

**Indexes:**
- `username` — UNIQUE INDEX

**Sensitive fields:**
- `password_hash` — Excluded from JSON serialization (`json:"-"`)

---

### api_credentials

**Purpose:** MTProto API credential configurations (API ID + API Hash pairs).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uint | PK, auto-increment | Primary key |
| display_name | string(128) | NOT NULL, indexed | User-friendly name |
| api_id | int32 | NOT NULL, indexed | Telegram API ID |
| encrypted_api_hash | string(512) | NOT NULL | Encrypted API hash. **Never exposed in API responses.** |
| api_hash_fingerprint | string(32) | | Display fingerprint: `first4...last4` |
| status | string(16) | NOT NULL, default "enabled", indexed | "enabled" or "disabled" |
| risk_policy | string(16) | NOT NULL, default "disabled" | "disabled", "enabled", or "confirm" |
| last_used_at | datetime | nullable | Last time this credential was used for login |
| created_at | datetime | NOT NULL | Record creation time |
| updated_at | datetime | NOT NULL | Last modification time |

**Indexes:**
- `display_name` — INDEX (for search)
- `api_id` — INDEX (for lookup)
- `status` — INDEX (for filtering)

**Sensitive fields:**
- `encrypted_api_hash` — Excluded from JSON serialization

**Display format:** `display_name · api_id_last4 · api_hash_first4...api_hash_last4`

---

### telegram_accounts

**Purpose:** Logged-in MTProto user accounts.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uint | PK, auto-increment | Primary key |
| api_credential_id | uint | NOT NULL, indexed, FK → api_credentials | Bound API credential |
| user_id | int64 | NOT NULL, indexed | Telegram user ID |
| phone_encrypted | string(512) | NOT NULL | Encrypted phone number. **Never exposed in API responses.** |
| phone_fingerprint | string(32) | | Display fingerprint (e.g., last 4 digits) |
| username | string(64) | indexed | Telegram username |
| first_name | string(128) | | First name |
| last_name | string(128) | | Last name |
| display_name | string(256) | | Computed display name |
| status | string(16) | NOT NULL, default "active", indexed | "active", "invalid", "logged_out", "restricted", "error" |
| is_premium | bool | NOT NULL, default false | Telegram Premium status |
| is_restricted | bool | NOT NULL, default false | Account restricted flag |
| is_scam | bool | NOT NULL, default false | Scam flag |
| is_fake | bool | NOT NULL, default false | Fake flag |
| last_sync_at | datetime | nullable | Last profile sync time |
| created_at | datetime | NOT NULL | Record creation time |
| updated_at | datetime | NOT NULL | Last modification time |

**Indexes:**
- `user_id` — INDEX
- `api_credential_id` — INDEX
- `username` — INDEX
- `status` — INDEX

**Sensitive fields:**
- `phone_encrypted` — Excluded from JSON serialization

---

### account_sessions

**Purpose:** Session file index. Maps Telegram accounts to their encrypted session files.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uint | PK, auto-increment | Primary key |
| telegram_account_id | uint | UNIQUE INDEX, NOT NULL, FK → telegram_accounts | One session per account |
| session_file_path | string(512) | NOT NULL | Relative path to encrypted session file. **Must not contain phone numbers.** |
| session_fingerprint | string(64) | | Hash of session file for integrity check |
| encryption_version | int | NOT NULL, default 1 | Encryption version (for future key rotation) |
| status | string(16) | NOT NULL, default "active" | "active", "invalid", "deleted", "error" |
| last_verified_at | datetime | nullable | Last time session was verified as valid |
| created_at | datetime | NOT NULL | Record creation time |
| updated_at | datetime | NOT NULL | Last modification time |

**Indexes:**
- `telegram_account_id` — UNIQUE INDEX (one session per account)

**Design note:** `telegram_account_id` uses a UNIQUE INDEX because each Telegram account has exactly one session. If an account is re-logged-in, the session file is overwritten and the record is updated.

**secret.key 影响：** `secret.key` 是所有加密数据的唯一解密密钥。如果密钥丢失或更换，旧的 Session 文件将无法解密，需要重新登录所有账号。这是预期行为，用户必须备份 `secret.key`。

---

### account_sync_snapshots

**Purpose:** Stores snapshots of synchronized account data (groups, channels, profile).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uint | PK, auto-increment | Primary key |
| telegram_account_id | uint | NOT NULL, indexed, FK → telegram_accounts | Parent account |
| snapshot_type | string(32) | NOT NULL | "groups", "channels", "profile" |
| encrypted_payload_path | string(512) | | Path to encrypted snapshot file. **Excluded from API responses.** |
| payload_summary | string(1024) | | Non-sensitive summary (e.g., "15 groups synced") |
| item_count | int | NOT NULL, default 0 | Number of items in snapshot |
| created_at | datetime | NOT NULL | Snapshot creation time |

**Indexes:**
- `telegram_account_id` — INDEX

**Sensitive fields:**
- `encrypted_payload_path` — Excluded from JSON serialization

---

### audit_logs

**Purpose:** Append-only operation audit trail.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uint | PK, auto-increment | Primary key |
| actor_type | string(32) | NOT NULL | "admin" or "system" |
| actor_id | uint | NOT NULL | ID of the actor (admin ID or 0 for system) |
| action | string(64) | NOT NULL, indexed | Action identifier (e.g., "api_credential.create") |
| resource_type | string(64) | NOT NULL | Resource type (e.g., "api_credential") |
| resource_id | uint | | Resource ID |
| risk_level | string(16) | NOT NULL, default "low" | "low", "medium", "high", "critical" |
| ip | string(45) | | Client IP (supports IPv6) |
| user_agent | string(512) | | Client user agent |
| message | string(1024) | | Human-readable description |
| metadata_json | string(4096) | | Additional context. **Must not contain sensitive raw data.** |
| created_at | datetime | NOT NULL, indexed | Log entry time |

**Indexes:**
- `action` — INDEX
- `created_at` — INDEX
- Composite: `(actor_type, actor_id)` — optional, for querying by actor

**Constraints:**
- Append-only: no UPDATE or DELETE via application
- `metadata_json` must not contain: API hashes, passwords, session content, verification codes, 2FA passwords

---

### system_settings

**Purpose:** Key-value system configuration storage.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| key | string(128) | UNIQUE INDEX, NOT NULL | Setting key |
| value | string(4096) | | Setting value (encrypted if is_sensitive) |
| value_type | string(32) | NOT NULL, default "string" | "string", "int", "bool", "json" |
| is_sensitive | bool | NOT NULL, default false | Whether value is encrypted |
| created_at | datetime | NOT NULL | Record creation time |
| updated_at | datetime | NOT NULL | Last modification time |

**Indexes:**
- `key` — UNIQUE INDEX

**Sensitive fields:**
- `value` when `is_sensitive = true` — encrypted at rest

**Example settings:**
- `site_name` → `"Atria"` (not sensitive)
- `session_timeout` → `"3600"` (not sensitive)
- `backup_encryption_key` → `(encrypted)` (sensitive)

---

## Sensitive Field Summary

| Table | Field | Protection |
|-------|-------|------------|
| admins | password_hash | Bcrypt/argon2id hash, excluded from JSON |
| api_credentials | encrypted_api_hash | Encrypted, excluded from JSON |
| telegram_accounts | phone_encrypted | Encrypted, excluded from JSON |
| account_sessions | session_file_path | No phone numbers in path |
| account_sync_snapshots | encrypted_payload_path | Encrypted file, excluded from JSON |
| audit_logs | metadata_json | No sensitive raw data allowed |
| system_settings | value (if sensitive) | Encrypted at rest |
