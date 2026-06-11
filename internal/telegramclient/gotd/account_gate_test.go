package gotd

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAccountGate_LockUnlock(t *testing.T) {
	gate := NewAccountGate()

	gate.Lock(1, "test")
	if !gate.IsLocked(1) {
		t.Error("账号 1 应该被锁定")
	}
	if gate.IsLocked(2) {
		t.Error("账号 2 不应该被锁定")
	}

	gate.Unlock(1)
	if gate.IsLocked(1) {
		t.Error("账号 1 解锁后不应该被锁定")
	}
}

func TestAccountGate_TryLock(t *testing.T) {
	gate := NewAccountGate()

	ok := gate.TryLock(1, "test")
	if !ok {
		t.Fatal("第一次 TryLock 应该成功")
	}

	// 第二次 TryLock 应该失败（已被占用）
	ok = gate.TryLock(1, "test2")
	if ok {
		t.Fatal("第二次 TryLock 应该失败")
	}

	gate.Unlock(1)

	// 解锁后应该可以再次获取
	ok = gate.TryLock(1, "test3")
	if !ok {
		t.Fatal("解锁后 TryLock 应该成功")
	}
	gate.Unlock(1)
}

func TestAccountGate_ConcurrentAccess(t *testing.T) {
	gate := NewAccountGate()

	var mu sync.Mutex
	owners := []string{}

	// 并发尝试锁定同一个 account
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			owner := fmt.Sprintf("goroutine_%d", id)
			gate.Lock(1, owner)
			mu.Lock()
			owners = append(owners, owner)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			gate.Unlock(1)
		}(i)
	}

	wg.Wait()

	// 所有 goroutine 都应该有机会获取锁
	if len(owners) != 10 {
		t.Errorf("期望 10 个 owner，实际 %d", len(owners))
	}
}

func TestAccountGate_AccountIsolation(t *testing.T) {
	gate := NewAccountGate()

	gate.Lock(1, "account1")
	defer gate.Unlock(1)

	// 不同 account 互不影响
	ok := gate.TryLock(2, "account2")
	if !ok {
		t.Error("不同 account 应该可以同时锁定")
	}
	gate.Unlock(2)
}
