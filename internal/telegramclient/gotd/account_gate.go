package gotd

import (
	"sync"
	"time"
)

// AccountGate 管理 per-account 的执行锁。
// 防止同一 account 的 REST 临时 client 和 Runtime long-lived client 并发运行。
// 这是过渡方案，后续应改为 runtime execution queue。
type AccountGate struct {
	gates sync.Map // accountID -> *accountLock
}

// accountLock 单个账号的执行锁。
type accountLock struct {
	mu       sync.Mutex
	owner    string // "runtime" or "rest"，用于调试
	lockedAt time.Time
}

// NewAccountGate 创建 AccountGate。
func NewAccountGate() *AccountGate {
	return &AccountGate{}
}

// Lock 获取指定账号的执行锁。阻塞直到获取成功。
func (g *AccountGate) Lock(accountID uint, owner string) {
	lock := g.getOrCreate(accountID)
	lock.mu.Lock()
	lock.owner = owner
	lock.lockedAt = time.Now()
}

// Unlock 释放指定账号的执行锁。
func (g *AccountGate) Unlock(accountID uint) {
	lock := g.getOrCreate(accountID)
	lock.owner = ""
	lock.mu.Unlock()
}

// TryLock 尝试获取指定账号的执行锁。如果已被占用，返回 false。
func (g *AccountGate) TryLock(accountID uint, owner string) bool {
	lock := g.getOrCreate(accountID)
	locked := lock.mu.TryLock()
	if locked {
		lock.owner = owner
		lock.lockedAt = time.Now()
	}
	return locked
}

// IsLocked 检查指定账号是否被锁定。
func (g *AccountGate) IsLocked(accountID uint) bool {
	lock := g.getOrCreate(accountID)
	locked := lock.mu.TryLock()
	if locked {
		lock.mu.Unlock()
		return false
	}
	return true
}

// getOrCreate 获取或创建指定账号的锁。
func (g *AccountGate) getOrCreate(accountID uint) *accountLock {
	val, ok := g.gates.Load(accountID)
	if ok {
		return val.(*accountLock)
	}
	lock := &accountLock{}
	actual, _ := g.gates.LoadOrStore(accountID, lock)
	return actual.(*accountLock)
}
