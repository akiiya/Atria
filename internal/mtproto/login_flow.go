package mtproto

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LoginFlow 表示登录流程状态。
type LoginFlow struct {
	ID                     string
	APICredentialID        uint
	APIID                  int // 存储 APIID，用于 SubmitCode/SubmitPassword 重建客户端
	PhoneEncrypted         string
	PhoneFingerprint       string
	State                  LoginState
	PhoneCodeHashEncrypted string
	ExpiresAt              time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// IsExpired 检查流程是否已过期。
func (f *LoginFlow) IsExpired() bool {
	return time.Now().After(f.ExpiresAt)
}

// FlowStore 定义登录流程存储接口。
type FlowStore interface {
	Create(ctx context.Context, flow LoginFlow) error
	Get(ctx context.Context, id string) (*LoginFlow, error)
	Update(ctx context.Context, flow LoginFlow) error
	Delete(ctx context.Context, id string) error
}

// MemoryFlowStore 是基于内存的 FlowStore 实现。
type MemoryFlowStore struct {
	mu    sync.RWMutex
	flows map[string]LoginFlow
}

// NewMemoryFlowStore 创建内存 FlowStore。
func NewMemoryFlowStore() *MemoryFlowStore {
	return &MemoryFlowStore{
		flows: make(map[string]LoginFlow),
	}
}

func (s *MemoryFlowStore) Create(ctx context.Context, flow LoginFlow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.flows[flow.ID]; exists {
		return fmt.Errorf("流程已存在: %s", flow.ID)
	}

	s.flows[flow.ID] = flow
	return nil
}

func (s *MemoryFlowStore) Get(ctx context.Context, id string) (*LoginFlow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	flow, exists := s.flows[id]
	if !exists {
		return nil, fmt.Errorf("流程不存在: %s", id)
	}

	if flow.IsExpired() {
		return nil, fmt.Errorf("流程已过期: %s", id)
	}

	return &flow, nil
}

func (s *MemoryFlowStore) Update(ctx context.Context, flow LoginFlow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.flows[flow.ID]; !exists {
		return fmt.Errorf("流程不存在: %s", flow.ID)
	}

	s.flows[flow.ID] = flow
	return nil
}

func (s *MemoryFlowStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.flows, id)
	return nil
}

// CleanupExpired 清理过期流程。
func (s *MemoryFlowStore) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, flow := range s.flows {
		if flow.IsExpired() {
			delete(s.flows, id)
			count++
		}
	}
	return count
}

// DefaultFlowTTL 是登录流程默认过期时间。
const DefaultFlowTTL = 5 * time.Minute

// NewLoginFlow 创建新的登录流程。
func NewLoginFlow(id string, credentialID uint, apiID int, phoneEncrypted, phoneFingerprint string) LoginFlow {
	now := time.Now()
	return LoginFlow{
		ID:               id,
		APICredentialID:  credentialID,
		APIID:            apiID,
		PhoneEncrypted:   phoneEncrypted,
		PhoneFingerprint: phoneFingerprint,
		State:            LoginStateWaitingPhone,
		ExpiresAt:        now.Add(DefaultFlowTTL),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
