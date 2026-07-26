import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { RealtimeClient } from '../ws'

/**
 * 可控的 WebSocket 替身。
 * 真实 WebSocket 在 jsdom 下无法建立连接，因此这里用替身驱动
 * onopen / onmessage / onclose，以覆盖存活检测与重连退避逻辑。
 */
class MockWebSocket {
  static instances: MockWebSocket[] = []

  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  closeCallCount = 0

  constructor(public url: string) {
    MockWebSocket.instances.push(this)
  }

  close() {
    this.closeCallCount++
    this.onclose?.()
  }

  // 测试辅助
  simulateOpen() { this.onopen?.() }
  simulateMessage(payload: unknown) { this.onmessage?.({ data: JSON.stringify(payload) }) }

  static reset() { MockWebSocket.instances = [] }
  static get latest() { return MockWebSocket.instances[MockWebSocket.instances.length - 1] }
}

describe('RealtimeClient 存活检测与重连', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    MockWebSocket.reset()
    vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket)
    vi.stubGlobal('location', { protocol: 'http:', host: 'localhost:8080' })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('连接建立后进入 connected 状态', () => {
    const onStateChange = vi.fn()
    const client = new RealtimeClient({ onEvent: vi.fn(), onStateChange })

    client.connect()
    MockWebSocket.latest.simulateOpen()

    expect(client.getState()).toBe('connected')
    client.close()
  })

  it('长时间无任何数据时主动关闭连接以触发重连', () => {
    const client = new RealtimeClient({ onEvent: vi.fn() })
    client.connect()

    const socket = MockWebSocket.latest
    socket.simulateOpen()
    expect(socket.closeCallCount).toBe(0)

    // 超过存活阈值（75s）后，检测周期（15s）应触发主动关闭
    vi.advanceTimersByTime(90_000)
    expect(socket.closeCallCount).toBeGreaterThan(0)

    client.close()
  })

  it('持续收到数据时不会误判为连接失效', () => {
    const onEvent = vi.fn()
    const client = new RealtimeClient({ onEvent })
    client.connect()

    const socket = MockWebSocket.latest
    socket.simulateOpen()

    // 每 30s 收到一次数据，模拟服务端心跳节奏
    for (let i = 0; i < 4; i++) {
      vi.advanceTimersByTime(30_000)
      socket.simulateMessage({ type: 'sync.done', event_id: `e${i}`, account_id: 1, created_at: '' })
    }

    expect(socket.closeCallCount).toBe(0)
    expect(onEvent).toHaveBeenCalledTimes(4)

    client.close()
  })

  it('close 之后停止存活检测，不再产生额外的关闭调用', () => {
    const client = new RealtimeClient({ onEvent: vi.fn() })
    client.connect()

    const socket = MockWebSocket.latest
    socket.simulateOpen()
    client.close()

    const countAfterClose = socket.closeCallCount
    vi.advanceTimersByTime(120_000)

    expect(socket.closeCallCount).toBe(countAfterClose)
  })

  it('连接断开后自动重连', () => {
    const client = new RealtimeClient({ onEvent: vi.fn() })
    client.connect()

    const first = MockWebSocket.latest
    first.simulateOpen()
    expect(MockWebSocket.instances).toHaveLength(1)

    // 模拟服务端断开
    first.onclose?.()
    expect(client.getState()).toBe('reconnecting')

    // 首次重连延迟 1s，加最多 ±25% 抖动，2s 足以覆盖
    vi.advanceTimersByTime(2_000)
    expect(MockWebSocket.instances.length).toBeGreaterThan(1)

    client.close()
  })

  it('重连延迟带抖动，避免所有客户端同时重连', () => {
    const delays = new Set<number>()

    // 多次运行，收集首次重连的实际延迟
    for (let run = 0; run < 12; run++) {
      MockWebSocket.reset()
      const client = new RealtimeClient({ onEvent: vi.fn() })
      client.connect()
      MockWebSocket.latest.simulateOpen()
      MockWebSocket.latest.onclose?.()

      // 以 50ms 为粒度推进，找出重连实际发生的时刻
      let elapsed = 0
      while (MockWebSocket.instances.length === 1 && elapsed < 3_000) {
        vi.advanceTimersByTime(50)
        elapsed += 50
      }
      delays.add(elapsed)
      client.close()
    }

    // 若无抖动，所有运行的延迟都会完全相同
    expect(delays.size).toBeGreaterThan(1)
  })

  it('重连成功后退避延迟重置', () => {
    const client = new RealtimeClient({ onEvent: vi.fn() })
    client.connect()

    // 断开两次，让退避增长
    MockWebSocket.latest.simulateOpen()
    MockWebSocket.latest.onclose?.()
    vi.advanceTimersByTime(2_000)
    MockWebSocket.latest.onclose?.()
    vi.advanceTimersByTime(4_000)

    // 成功连上后退避应重置，下一次断开仍在 ~1s 量级重连
    const beforeReconnect = MockWebSocket.instances.length
    MockWebSocket.latest.simulateOpen()
    MockWebSocket.latest.onclose?.()
    vi.advanceTimersByTime(2_000)

    expect(MockWebSocket.instances.length).toBeGreaterThan(beforeReconnect)
    client.close()
  })

  it('主动 close 后不再重连', () => {
    const client = new RealtimeClient({ onEvent: vi.fn() })
    client.connect()
    MockWebSocket.latest.simulateOpen()

    client.close()
    const countAfterClose = MockWebSocket.instances.length

    vi.advanceTimersByTime(60_000)
    expect(MockWebSocket.instances).toHaveLength(countAfterClose)
  })
})
