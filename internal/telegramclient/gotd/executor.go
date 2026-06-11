package gotd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/telegramclient"
)

// ExecuteFunc 是在 runtime client 上执行的函数。
// gotd 类型 (*tg.Client) 只在此函数签名中出现，不暴露到外部包。
type ExecuteFunc func(ctx context.Context, api *tg.Client) error

// executeRequest 表示一个待执行的请求。
type executeRequest struct {
	ctx      context.Context
	fn       ExecuteFunc
	resultCh chan error
}

// RuntimeExecutor 在 runtime 的 gotd client 上串行执行 API 调用。
// 每个 AccountRuntime 持有一个 RuntimeExecutor。
// REST 请求通过 executor 使用 runtime 的 long-lived client，避免创建临时 client。
type RuntimeExecutor struct {
	requestCh chan executeRequest
	doneCh    chan struct{} // 关闭信号
	logger    *slog.Logger
	accountID uint
	closed    bool
	mu        sync.Mutex
}

// NewRuntimeExecutor 创建 RuntimeExecutor。
func NewRuntimeExecutor(accountID uint, bufferSize int, logger *slog.Logger) *RuntimeExecutor {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &RuntimeExecutor{
		requestCh: make(chan executeRequest, bufferSize),
		doneCh:    make(chan struct{}),
		logger:    logger,
		accountID: accountID,
	}
}

// Execute 在 runtime client 上执行一个 API 调用。
// 串行执行，支持 context cancellation 和 timeout。
// 如果 queue 满，返回 ErrorCodeRuntimeQueueFull。
func (e *RuntimeExecutor) Execute(ctx context.Context, fn ExecuteFunc) error {
	// 检查是否已关闭
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return telegramclient.NewError(telegramclient.ErrorCodeRuntimeStopped, "运行时已停止")
	}
	e.mu.Unlock()

	// 检查 context 是否已取消
	if ctx.Err() != nil {
		return telegramclient.WrapError(telegramclient.ErrorCodeRuntimeExecuteTimeout, "请求已取消", ctx.Err())
	}

	resultCh := make(chan error, 1)
	req := executeRequest{
		ctx:      ctx,
		fn:       fn,
		resultCh: resultCh,
	}

	// 尝试发送到 queue
	select {
	case e.requestCh <- req:
		// 成功入队
	case <-e.doneCh:
		return telegramclient.NewError(telegramclient.ErrorCodeRuntimeStopped, "运行时已停止")
	default:
		// queue 满
		return telegramclient.NewError(telegramclient.ErrorCodeRuntimeQueueFull, "运行时请求队列已满，请稍后重试")
	}

	// 等待结果或 context 取消
	select {
	case err := <-resultCh:
		return err
	case <-ctx.Done():
		return telegramclient.WrapError(telegramclient.ErrorCodeRuntimeExecuteTimeout, "请求超时或已取消", ctx.Err())
	}
}

// Run 消费循环。在 client.Run() 回调中启动。
// 阻塞直到 doneCh 关闭或 ctx 取消。
// api 是 runtime client 的 *tg.Client。
func (e *RuntimeExecutor) Run(ctx context.Context, api *tg.Client) {
	for {
		select {
		case <-ctx.Done():
			e.drainPending(ctx.Err())
			return
		case <-e.doneCh:
			e.drainPending(fmt.Errorf("executor shutdown"))
			return
		case req := <-e.requestCh:
			e.executeOne(req, api)
		}
	}
}

// executeOne 执行单个请求，带 panic recover。
func (e *RuntimeExecutor) executeOne(req executeRequest, api *tg.Client) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("executor panic: %v", r)
			e.logger.Error("executor 执行 panic",
				"account_id", e.accountID,
				"panic", r,
			)
			select {
			case req.resultCh <- err:
			default:
			}
		}
	}()

	// 创建带 timeout 的 context（如果原始 ctx 没有 deadline）
	execCtx := req.ctx
	if _, ok := req.ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(req.ctx, 30*time.Second)
		defer cancel()
	}

	err := req.fn(execCtx, api)

	// 发送结果
	select {
	case req.resultCh <- err:
	default:
		// resultCh 已满（不应该发生，buffer=1）
	}
}

// drainPending 排空所有等待中的请求，返回错误。
func (e *RuntimeExecutor) drainPending(cause error) {
	err := telegramclient.WrapError(telegramclient.ErrorCodeRuntimeStopped, "运行时已停止", cause)
	for {
		select {
		case req := <-e.requestCh:
			select {
			case req.resultCh <- err:
			default:
			}
		default:
			return
		}
	}
}

// Close 关闭 executor，排空所有等待中的请求。
func (e *RuntimeExecutor) Close() {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	close(e.doneCh)
}

// PendingCount 返回当前等待中的请求数（用于测试/监控）。
func (e *RuntimeExecutor) PendingCount() int {
	return len(e.requestCh)
}
