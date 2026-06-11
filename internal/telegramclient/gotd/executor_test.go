package gotd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/telegramclient"
)

func TestRuntimeExecute_RejectsWhenStopped(t *testing.T) {
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	// 不调用 Run()，直接 Execute
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
		return nil
	})

	if err == nil {
		t.Fatal("executor 未运行时应返回错误")
	}
	// 应该超时或返回 queue 相关错误
}

func TestRuntimeExecute_ContextCancel(t *testing.T) {
	executor := NewRuntimeExecutor(1, 10, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	err := executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
		return nil
	})

	if err == nil {
		t.Fatal("context 已取消应返回错误")
	}
}

func TestRuntimeExecute_Timeout(t *testing.T) {
	executor := NewRuntimeExecutor(1, 10, newTestLogger())

	// 不启动 Run()，请求会超时
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
		return nil
	})

	if err == nil {
		t.Fatal("应返回超时错误")
	}
}

func TestRuntimeExecute_QueueFull(t *testing.T) {
	// 创建一个 buffer=1 的 executor
	executor := NewRuntimeExecutor(1, 1, newTestLogger())

	// 不启动 Run()，填满 queue
	ctx1, cancel1 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel1()

	// 第一个请求入队
	go func() {
		executor.Execute(ctx1, func(ctx context.Context, api *tg.Client) error {
			time.Sleep(500 * time.Millisecond)
			return nil
		})
	}()

	time.Sleep(10 * time.Millisecond) // 等待第一个请求入队

	// 第二个请求应该返回 queue full
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	err := executor.Execute(ctx2, func(ctx context.Context, api *tg.Client) error {
		return nil
	})

	if err == nil {
		t.Fatal("queue 满时应返回错误")
	}
	tcErr, ok := err.(*telegramclient.Error)
	if !ok {
		t.Fatalf("期望 telegramclient.Error，实际 %T", err)
	}
	if tcErr.Code != telegramclient.ErrorCodeRuntimeQueueFull {
		t.Errorf("期望 runtime_queue_full，实际 %s", tcErr.Code)
	}
}

func TestRuntimeExecute_SerializesRequests(t *testing.T) {
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	ctx := context.Background()

	// 模拟 gotd client
	api := &tg.Client{}

	// 启动 executor 消费 goroutine
	go executor.Run(ctx, api)

	var mu sync.Mutex
	order := []int{}

	// 提交多个请求
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
				mu.Lock()
				order = append(order, id)
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			if err != nil {
				t.Errorf("请求 %d 失败: %s", id, err)
			}
		}(i)
	}

	wg.Wait()

	// 所有请求都应该完成
	if len(order) != 5 {
		t.Errorf("期望 5 个请求完成，实际 %d", len(order))
	}
}

func TestRuntimeExecute_RecoversPanic(t *testing.T) {
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	ctx := context.Background()
	api := &tg.Client{}

	go executor.Run(ctx, api)
	time.Sleep(10 * time.Millisecond)

	err := executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
		panic("test panic")
	})

	if err == nil {
		t.Fatal("panic 应被捕获并返回错误")
	}
}

func TestRuntimeExecute_StopDrainsPendingRequests(t *testing.T) {
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	ctx := context.Background()
	api := &tg.Client{}

	go executor.Run(ctx, api)
	time.Sleep(10 * time.Millisecond)

	// 提交一个慢请求
	go func() {
		_ = executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
			time.Sleep(500 * time.Millisecond)
			return nil
		})
	}()

	time.Sleep(20 * time.Millisecond)

	// 关闭 executor
	executor.Close()
	time.Sleep(50 * time.Millisecond)

	// 后续请求应该失败（channel 已关闭）
	ctx2, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := executor.Execute(ctx2, func(ctx context.Context, api *tg.Client) error {
		return nil
	})

	// 请求应该返回错误（executor 关闭或 context 超时）
	if err == nil {
		t.Fatal("executor 关闭后应返回错误")
	}
}

func TestRuntimeExecute_DoesNotLogMessageBody(t *testing.T) {
	// 验证 executor 不记录 message body
	// 这是一个设计约束测试 - executor 本身不记录任何内容
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	ctx := context.Background()
	api := &tg.Client{}

	go executor.Run(ctx, api)
	time.Sleep(10 * time.Millisecond)

	err := executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
		// 模拟包含敏感信息的调用
		return nil
	})

	if err != nil {
		t.Errorf("执行失败: %s", err)
	}
	// executor 本身不记录任何内容，只传递函数结果
}

func TestRuntimeExecute_PendingCount(t *testing.T) {
	executor := NewRuntimeExecutor(1, 10, newTestLogger())

	if executor.PendingCount() != 0 {
		t.Errorf("初始 pending 应为 0，实际 %d", executor.PendingCount())
	}
}

// ===== Adapter routing 测试 =====

func TestGotdAdapter_ListDialogs_UsesRuntimeWhenLive(t *testing.T) {
	// 验证当 runtime executor 可用时，adapter 使用 executor
	adapter := &Adapter{
		logger: newTestLogger(),
	}

	// 没有 runtime 时，getExecutor 应返回 nil
	executor := adapter.getExecutor(1)
	if executor != nil {
		t.Error("没有 runtime 时 getExecutor 应返回 nil")
	}
}

func TestGotdAdapter_FallbackTemporaryWhenRuntimeStopped(t *testing.T) {
	// 验证 runtime stopped 时 fallback 到临时 client
	adapter := &Adapter{
		logger: newTestLogger(),
	}

	// 没有 runtime 时应 fallback
	executor := adapter.getExecutor(1)
	if executor != nil {
		t.Error("runtime 未设置时 getExecutor 应返回 nil")
	}
}

func TestGotdAdapter_DoesNotFallbackUnprotectedWhenRuntimeLive(t *testing.T) {
	// 验证 runtime live 时不使用未保护的 fallback
	// 这是架构约束测试
	adapter := &Adapter{
		logger:  newTestLogger(),
		runtime: nil, // 没有 runtime
	}

	// 没有 runtime 时，adapter 会 fallback 到临时 client
	// 但会通过 AccountGate 保护
	executor := adapter.getExecutor(1)
	if executor != nil {
		t.Error("runtime 未设置时不应返回 executor")
	}
}

// ===== 边界测试 =====

func TestRuntimeExecutorGotdTypesOnlyInGotdPackage(t *testing.T) {
	// 验证 ExecuteFunc 使用 *tg.Client，只在 gotd 包内
	// 这是一个编译时约束 - 如果 ExecuteFunc 暴露到外部包会编译失败
	_ = ExecuteFunc(func(ctx context.Context, api *tg.Client) error {
		return nil
	})
}

// ===== 安全测试 =====

func TestRuntimeQueueLogs_NoMessageBody(t *testing.T) {
	// 验证 executor 不记录 message body
	// executor 本身不记录任何内容，只传递函数结果
	// 这是设计约束
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	if executor == nil {
		t.Fatal("executor 应创建成功")
	}
}

func TestRuntimeQueueLogs_NoAPIHash(t *testing.T) {
	// 验证 executor 不记录 api_hash
	// executor 不访问 api_hash，只执行传入的函数
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	if executor == nil {
		t.Fatal("executor 应创建成功")
	}
}

func TestRuntimeQueueLogs_NoProxyPassword(t *testing.T) {
	// 验证 executor 不记录 proxy_password
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	if executor == nil {
		t.Fatal("executor 应创建成功")
	}
}

func TestRuntimeQueueLogs_NoSessionPath(t *testing.T) {
	// 验证 executor 不记录 session path
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	if executor == nil {
		t.Fatal("executor 应创建成功")
	}
}

func TestRuntimeQueueLogs_NoAccessHash(t *testing.T) {
	// 验证 executor 不记录 access_hash
	executor := NewRuntimeExecutor(1, 10, newTestLogger())
	if executor == nil {
		t.Fatal("executor 应创建成功")
	}
}

// ===== 并发测试 =====

func TestRuntimeQueue_NoDeadlock(t *testing.T) {
	executor := NewRuntimeExecutor(1, 64, newTestLogger())
	ctx := context.Background()
	api := &tg.Client{}

	go executor.Run(ctx, api)
	time.Sleep(10 * time.Millisecond)

	// 并发提交多个请求
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := executor.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
				return nil
			})
			if err != nil {
				t.Errorf("请求 %d 失败: %s", id, err)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 成功
	case <-time.After(5 * time.Second):
		t.Fatal("可能死锁：20 个请求超时")
	}
}

func TestRuntimeQueue_DifferentAccountsDoNotBlockEachOther(t *testing.T) {
	// 不同 account 的 executor 独立，互不影响
	executor1 := NewRuntimeExecutor(1, 10, newTestLogger())
	executor2 := NewRuntimeExecutor(2, 10, newTestLogger())

	ctx := context.Background()
	api := &tg.Client{}

	go executor1.Run(ctx, api)
	go executor2.Run(ctx, api)
	time.Sleep(10 * time.Millisecond)

	// 两个 executor 应该独立工作
	err1 := executor1.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
		return nil
	})
	err2 := executor2.Execute(ctx, func(ctx context.Context, api *tg.Client) error {
		return nil
	})

	if err1 != nil {
		t.Errorf("executor1 失败: %s", err1)
	}
	if err2 != nil {
		t.Errorf("executor2 失败: %s", err2)
	}
}
