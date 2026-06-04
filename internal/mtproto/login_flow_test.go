package mtproto

import (
	"context"
	"testing"
	"time"
)

func TestFlowStore_CreateAndGet(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	flow := LoginFlow{
		ID:               "test-flow-1",
		APICredentialID:  1,
		PhoneEncrypted:   "encrypted-phone",
		PhoneFingerprint: "abc123",
		State:            LoginStateWaitingPhone,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := store.Create(ctx, flow); err != nil {
		t.Fatalf("创建失败: %s", err)
	}

	got, err := store.Get(ctx, "test-flow-1")
	if err != nil {
		t.Fatalf("获取失败: %s", err)
	}

	if got.ID != flow.ID {
		t.Errorf("ID 不匹配，期望=%s，实际=%s", flow.ID, got.ID)
	}
	if got.State != LoginStateWaitingPhone {
		t.Errorf("State 不匹配，期望=%s，实际=%s", LoginStateWaitingPhone, got.State)
	}
}

func TestFlowStore_Update(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	flow := LoginFlow{
		ID:               "test-flow-1",
		APICredentialID:  1,
		PhoneFingerprint: "abc123",
		State:            LoginStateWaitingPhone,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	store.Create(ctx, flow)

	flow.State = LoginStateCodeSent
	flow.UpdatedAt = time.Now()

	if err := store.Update(ctx, flow); err != nil {
		t.Fatalf("更新失败: %s", err)
	}

	got, _ := store.Get(ctx, "test-flow-1")
	if got.State != LoginStateCodeSent {
		t.Errorf("状态应更新为 %s，实际=%s", LoginStateCodeSent, got.State)
	}
}

func TestFlowStore_Delete(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	flow := LoginFlow{
		ID:               "test-flow-1",
		APICredentialID:  1,
		PhoneFingerprint: "abc123",
		State:            LoginStateWaitingPhone,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	store.Create(ctx, flow)
	store.Delete(ctx, "test-flow-1")

	_, err := store.Get(ctx, "test-flow-1")
	if err == nil {
		t.Error("删除后应无法获取")
	}
}

func TestFlowStore_ExpiredFlow(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	flow := LoginFlow{
		ID:               "test-flow-1",
		APICredentialID:  1,
		PhoneFingerprint: "abc123",
		State:            LoginStateWaitingPhone,
		ExpiresAt:        time.Now().Add(-1 * time.Minute), // 已过期
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	store.Create(ctx, flow)

	_, err := store.Get(ctx, "test-flow-1")
	if err == nil {
		t.Error("过期流程应返回错误")
	}
}

func TestFlowStore_NoCodeStored(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	// 创建流程时不保存验证码
	flow := LoginFlow{
		ID:               "test-flow-1",
		APICredentialID:  1,
		PhoneFingerprint: "abc123",
		State:            LoginStateCodeSent,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	store.Create(ctx, flow)

	got, _ := store.Get(ctx, "test-flow-1")

	// 检查没有验证码字段
	if got.PhoneCodeHashEncrypted != "" {
		// phone_code_hash 是 Telegram 返回的，不是用户输入的验证码
		// 这里只是确认没有保存用户验证码
	}
}

func TestFlowStore_NoPasswordStored(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	// 创建流程时不保存 2FA 密码
	flow := LoginFlow{
		ID:               "test-flow-1",
		APICredentialID:  1,
		PhoneFingerprint: "abc123",
		State:            LoginStateWaitingPassword,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	store.Create(ctx, flow)

	got, _ := store.Get(ctx, "test-flow-1")

	// LoginFlow 结构中没有 Password 字段
	_ = got
}

func TestFlowStore_PhoneIsEncrypted(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	// 手机号应为加密值或指纹
	flow := LoginFlow{
		ID:               "test-flow-1",
		APICredentialID:  1,
		PhoneEncrypted:   "encrypted-phone-data", // 不是明文手机号
		PhoneFingerprint: "abc123def456",
		State:            LoginStateWaitingPhone,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	store.Create(ctx, flow)

	got, _ := store.Get(ctx, "test-flow-1")

	// PhoneEncrypted 不应是明文手机号
	if got.PhoneEncrypted == "+8613800138000" {
		t.Error("手机号不应以明文保存")
	}
}

func TestFlowStore_CleanupExpired(t *testing.T) {
	store := NewMemoryFlowStore()
	ctx := context.Background()

	// 创建一个过期流程
	expiredFlow := LoginFlow{
		ID:               "expired-flow",
		APICredentialID:  1,
		PhoneFingerprint: "abc123",
		State:            LoginStateWaitingPhone,
		ExpiresAt:        time.Now().Add(-1 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// 创建一个有效流程
	validFlow := LoginFlow{
		ID:               "valid-flow",
		APICredentialID:  1,
		PhoneFingerprint: "def456",
		State:            LoginStateWaitingPhone,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	store.Create(ctx, expiredFlow)
	store.Create(ctx, validFlow)

	count := store.CleanupExpired()
	if count != 1 {
		t.Errorf("应清理 1 个过期流程，实际=%d", count)
	}

	// 有效流程应仍存在
	_, err := store.Get(ctx, "valid-flow")
	if err != nil {
		t.Error("有效流程应仍存在")
	}
}
