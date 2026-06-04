# Atria — Project Scope

## 1. Product Positioning

Atria is a lightweight, self-hosted MTProto multi-account session management panel. It manages multiple Telegram-compatible MTProto user sessions, API credential profiles, account metadata, and audit logs through a secure embedded web interface.

Atria is **not** affiliated with Telegram. It is **not** a spam, growth-hacking, scraping, or platform-abuse tool.

## 2. Phase 1 — Allowed Features

The following features are planned for Phase 1 (initial release):

### Authentication & Administration
- First-time administrator initialization (no default credentials)
- Administrator login / logout
- Password change (bcrypt or argon2id hashing)

### API Credential Management
- Add / edit / disable / delete API credentials
- Display name, API ID, encrypted API hash
- Per-credential risk policy (disabled / enabled / confirm)
- Quick-switch selector in the UI

### MTProto Account Management
- Telegram user login flow (phone → code → 2FA → encrypted session)
- Account list with status, user ID, phone (masked), username, name
- Account status detection (active, banned, restricted, scam, fake)
- Premium flag detection
- Logout / session removal

### Session Management
- Encrypted session file storage (local filesystem)
- Session index in database (no raw session content)
- Automatic secret key generation on first run

### Account Information Sync
- User profile metadata sync (user_id, phone, username, name, premium, flags)
- Group / channel read-only sync (planned, not implemented in Phase 1)

### Audit & Security
- Operation audit logging (actor, action, resource, IP, user agent)
- Risk policy enforcement
- CSRF protection
- Secure cookie configuration

### System
- System settings page
- Security information page
- Single-binary deployment with embedded web assets

## 3. Explicit Non-Goals

Atria explicitly does **not** implement and will **not** implement:

- Bulk messaging / spam
- Bulk member inviting
- Bulk group joining or leaving
- Automated account nurturing ("farming")
- Phone number verification code platforms (接码平台)
- Account trading or marketplace features
- Platform limit bypass or evasion
- Follower/view/engagement inflation
- Automated harassment or unsolicited contact
- Any feature designed to circumvent Telegram's terms of service

## 4. Risk Policy System

Each API credential has a risk policy field:

| Policy | Behavior |
|--------|----------|
| `disabled` | High-risk operations are blocked (default) |
| `enabled` | High-risk operations are allowed |
| `confirm` | High-risk operations require explicit confirmation |

**Phase 1 scope:** Model, configuration, audit fields, and UI placeholders only. No high-risk operations are implemented.

All future sensitive operations must pass through:
1. Risk policy check
2. Confirmation prompt (if policy is `confirm`)
3. Audit logging
4. Rate limiting

## 5. Future Expansion Directions

These are potential future features (not committed for Phase 1):

- Group / channel read-only information sync
- File upload/download capability (via MTProto)
- Message read-only access (with strict risk controls)
- Multi-language UI
- Configuration file support (YAML/TOML)
- Backup / restore utilities
- Prometheus metrics endpoint
- Docker image and deployment templates
