package telegramclient

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// EventBus 管理 per-account 的事件订阅和发布。
// 并发安全，慢 subscriber 不阻塞 publisher。
type EventBus struct {
	mu     sync.RWMutex
	subs   map[uint][]*subscription
	nextID atomic.Int64
	logger *slog.Logger
}

// subscription 表示一个订阅。
type subscription struct {
	id        string
	accountID uint
	bus       *EventBus
	sink      UpdateSink
	ch        chan UpdateEvent
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewEventBus 创建 EventBus。
func NewEventBus(logger *slog.Logger) *EventBus {
	return &EventBus{
		subs:   make(map[uint][]*subscription),
		logger: logger,
	}
}

// Subscribe 订阅指定账号的更新事件。
// 返回 Subscription，调用 Close() 取消订阅。
func (b *EventBus) Subscribe(accountID uint, sink UpdateSink) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := fmt.Sprintf("sub_%d_%d", accountID, b.nextID.Add(1))
	ctx, cancel := context.WithCancel(context.Background())

	sub := &subscription{
		id:        id,
		accountID: accountID,
		bus:       b,
		sink:      sink,
		ch:        make(chan UpdateEvent, 100),
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	b.subs[accountID] = append(b.subs[accountID], sub)

	// 启动 subscriber goroutine
	go b.drainSubscriber(ctx, sub)

	b.logger.Debug("订阅创建", "account_id", accountID, "sub_id", id)
	return sub, nil
}

// Publish 发布事件到指定账号的所有订阅者。
// 非阻塞：如果 subscriber 的 channel 满，事件会被丢弃并记录警告。
func (b *EventBus) Publish(accountID uint, event UpdateEvent) {
	b.mu.RLock()
	subs := b.subs[accountID]
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- event:
			// 成功发送
		default:
			// channel 满，丢弃事件
			b.logger.Warn("subscriber channel 满，丢弃事件",
				"sub_id", sub.id,
				"account_id", accountID,
				"event_type", event.Type,
			)
		}
	}
}

// Unsubscribe 移除指定账号的所有订阅。
func (b *EventBus) Unsubscribe(accountID uint) {
	b.mu.Lock()
	subs := b.subs[accountID]
	delete(b.subs, accountID)
	b.mu.Unlock()

	for _, sub := range subs {
		sub.cancel()
		<-sub.done
	}
	b.logger.Debug("账号订阅已移除", "account_id", accountID)
}

// Close 关闭 EventBus，取消所有订阅。
func (b *EventBus) Close() {
	b.mu.Lock()
	allSubs := make(map[uint][]*subscription)
	for k, v := range b.subs {
		allSubs[k] = v
	}
	b.subs = make(map[uint][]*subscription)
	b.mu.Unlock()

	for _, subs := range allSubs {
		for _, sub := range subs {
			sub.cancel()
			<-sub.done
		}
	}
}

// SubscriptionCount 返回指定账号的订阅数量（用于测试）。
func (b *EventBus) SubscriptionCount(accountID uint) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs[accountID])
}

// drainSubscriber 持续从 channel 读取事件并发送到 sink。
func (b *EventBus) drainSubscriber(ctx context.Context, sub *subscription) {
	defer close(sub.done)
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-sub.ch:
			if err := sub.sink.Send(event); err != nil {
				b.logger.Warn("发送事件到 subscriber 失败",
					"sub_id", sub.id,
					"error", err,
				)
			}
		}
	}
}

// Close 关闭订阅并从 EventBus 移除。
func (s *subscription) Close() error {
	s.cancel()
	<-s.done

	// 从 bus 中移除自己
	s.bus.mu.Lock()
	subs := s.bus.subs[s.accountID]
	for i, sub := range subs {
		if sub.id == s.id {
			s.bus.subs[s.accountID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(s.bus.subs[s.accountID]) == 0 {
		delete(s.bus.subs, s.accountID)
	}
	s.bus.mu.Unlock()

	return nil
}

// channelSink 是 UpdateSink 的简单实现，将事件发送到 channel。
type channelSink struct {
	ch chan UpdateEvent
}

// NewChannelSink 创建一个基于 channel 的 UpdateSink。
func NewChannelSink(bufferSize int) (UpdateSink, <-chan UpdateEvent) {
	ch := make(chan UpdateEvent, bufferSize)
	return &channelSink{ch: ch}, ch
}

func (s *channelSink) Send(event UpdateEvent) error {
	select {
	case s.ch <- event:
		return nil
	default:
		return fmt.Errorf("channel full")
	}
}
