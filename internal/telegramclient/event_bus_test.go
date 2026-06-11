package telegramclient_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/user/atria/internal/telegramclient"
)

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := telegramclient.NewEventBus(slog.Default())
	defer bus.Close()

	sink, ch := telegramclient.NewChannelSink(10)
	_, err := bus.Subscribe(1, sink)
	if err != nil {
		t.Fatalf("Subscribe 失败: %s", err)
	}

	event := telegramclient.UpdateEvent{
		EventID:   "test_1",
		AccountID: 1,
		Type:      telegramclient.EventMessageNew,
		PeerRef:   "u_123",
		CreatedAt: time.Now(),
	}

	bus.Publish(1, event)

	select {
	case received := <-ch:
		if received.EventID != "test_1" {
			t.Errorf("期望 EventID=test_1，实际 %s", received.EventID)
		}
		if received.Type != telegramclient.EventMessageNew {
			t.Errorf("期望 Type=message.new，实际 %s", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("超时：未收到事件")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := telegramclient.NewEventBus(slog.Default())
	defer bus.Close()

	sink, _ := telegramclient.NewChannelSink(10)
	sub, err := bus.Subscribe(1, sink)
	if err != nil {
		t.Fatalf("Subscribe 失败: %s", err)
	}

	if bus.SubscriptionCount(1) != 1 {
		t.Errorf("期望 1 个订阅，实际 %d", bus.SubscriptionCount(1))
	}

	sub.Close()

	// 等待 goroutine 退出
	time.Sleep(50 * time.Millisecond)

	if bus.SubscriptionCount(1) != 0 {
		t.Errorf("期望 0 个订阅，实际 %d", bus.SubscriptionCount(1))
	}
}

func TestEventBus_SlowSubscriberDoesNotBlock(t *testing.T) {
	bus := telegramclient.NewEventBus(slog.Default())
	defer bus.Close()

	// 创建一个慢 subscriber（buffer=2）
	sink, ch := telegramclient.NewChannelSink(2)
	bus.Subscribe(1, sink)

	// 发布超过 buffer 大小的事件
	for i := 0; i < 10; i++ {
		bus.Publish(1, telegramclient.UpdateEvent{
			EventID:   fmt.Sprintf("test_%d", i),
			AccountID: 1,
			Type:      telegramclient.EventMessageNew,
			CreatedAt: time.Now(),
		})
	}

	// 应该不阻塞，只有 buffer 大小的事件被接收
	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}
done:
	if received > 2 {
		t.Errorf("慢 subscriber 不应收到超过 buffer 的事件，实际 %d", received)
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := telegramclient.NewEventBus(slog.Default())
	defer bus.Close()

	sink1, ch1 := telegramclient.NewChannelSink(10)
	sink2, ch2 := telegramclient.NewChannelSink(10)
	bus.Subscribe(1, sink1)
	bus.Subscribe(1, sink2)

	event := telegramclient.UpdateEvent{
		EventID:   "test_multi",
		AccountID: 1,
		Type:      telegramclient.EventMessageNew,
		CreatedAt: time.Now(),
	}

	bus.Publish(1, event)

	// 两个 subscriber 都应该收到
	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("subscriber 1 超时")
	}

	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("subscriber 2 超时")
	}
}

func TestEventBus_AccountIsolation(t *testing.T) {
	bus := telegramclient.NewEventBus(slog.Default())
	defer bus.Close()

	sink1, ch1 := telegramclient.NewChannelSink(10)
	sink2, ch2 := telegramclient.NewChannelSink(10)
	bus.Subscribe(1, sink1)
	bus.Subscribe(2, sink2)

	// 只发布到 account 1
	bus.Publish(1, telegramclient.UpdateEvent{
		EventID:   "test_isolation",
		AccountID: 1,
		Type:      telegramclient.EventMessageNew,
		CreatedAt: time.Now(),
	})

	// account 1 应该收到
	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("account 1 subscriber 超时")
	}

	// account 2 不应该收到
	select {
	case <-ch2:
		t.Fatal("account 2 不应收到事件")
	case <-time.After(100 * time.Millisecond):
		// 正确：没有事件
	}
}

func TestEventBus_Close(t *testing.T) {
	bus := telegramclient.NewEventBus(slog.Default())

	sink, _ := telegramclient.NewChannelSink(10)
	bus.Subscribe(1, sink)
	bus.Subscribe(2, sink)

	bus.Close()

	// Close 后不应该 panic
	bus.Publish(1, telegramclient.UpdateEvent{
		EventID: "after_close",
	})
}
