export type WSState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected' | 'error'

export interface RealtimeEvent {
  type: string
  event_id: string
  account_id: number
  peer_ref?: string
  created_at: string
  payload?: unknown
}

export interface RealtimeClientOptions {
  onEvent: (event: RealtimeEvent) => void
  onStateChange?: (state: WSState) => void
}

const INITIAL_RECONNECT_DELAY = 1000
const MAX_RECONNECT_DELAY = 30000
const RECONNECT_BACKOFF_FACTOR = 2
// 抖动比例：重连延迟随机浮动 ±25%，避免服务端重启后所有客户端同时重连
const RECONNECT_JITTER = 0.25

// 服务端每 30s 发一次 ping。浏览器会在协议层自动回 pong，JS 无法感知，
// 因此半开连接（TCP 未断但数据不通）对前端是不可见的。
// 这里用「最后一次收到任何数据的时间」做存活判断：超过阈值即视为连接已死，
// 主动关闭并触发重连，而不是干等 OS 的 TCP 超时。
const LIVENESS_TIMEOUT = 75_000
const LIVENESS_CHECK_INTERVAL = 15_000

export class RealtimeClient {
  private ws: WebSocket | null = null
  private state: WSState = 'disconnected'
  private reconnectDelay = INITIAL_RECONNECT_DELAY
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private livenessTimer: ReturnType<typeof setInterval> | null = null
  private lastActivityAt = 0
  private closed = false
  private options: RealtimeClientOptions

  constructor(options: RealtimeClientOptions) {
    this.options = options
  }

  connect(): void {
    if (this.closed) return
    if (this.ws) return

    this.setState('connecting')

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/api/realtime/ws`

    try {
      this.ws = new WebSocket(url)
    } catch {
      this.setState('error')
      this.scheduleReconnect()
      return
    }

    this.ws.onopen = () => {
      this.setState('connected')
      this.reconnectDelay = INITIAL_RECONNECT_DELAY
      this.lastActivityAt = Date.now()
      this.startLivenessCheck()
    }

    this.ws.onmessage = (event) => {
      this.lastActivityAt = Date.now()
      try {
        const data = JSON.parse(event.data) as RealtimeEvent
        this.options.onEvent(data)
      } catch {
        // 忽略无法解析的消息
      }
    }

    this.ws.onclose = () => {
      this.ws = null
      this.stopLivenessCheck()
      if (!this.closed) {
        this.setState('reconnecting')
        this.scheduleReconnect()
      } else {
        this.setState('disconnected')
      }
    }

    this.ws.onerror = () => {
      // onerror 之后会触发 onclose，所以这里只记录状态
      this.setState('error')
    }
  }

  close(): void {
    this.closed = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.stopLivenessCheck()
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
    this.setState('disconnected')
  }

  getState(): WSState {
    return this.state
  }

  private setState(state: WSState): void {
    if (this.state === state) return
    this.state = state
    this.options.onStateChange?.(state)
  }

  private scheduleReconnect(): void {
    if (this.closed) return
    if (this.reconnectTimer) return

    // 加抖动：多个客户端不会在服务端恢复的同一瞬间一起重连
    const jitter = 1 + (Math.random() * 2 - 1) * RECONNECT_JITTER
    const delay = Math.round(this.reconnectDelay * jitter)

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, delay)

    // 指数退避
    this.reconnectDelay = Math.min(
      this.reconnectDelay * RECONNECT_BACKOFF_FACTOR,
      MAX_RECONNECT_DELAY
    )
  }

  /**
   * 启动存活检测。
   * 服务端 ping 由浏览器自动应答，JS 收不到事件，因此改为检测
   * 「距离上次收到任何数据是否超时」。超时说明连接已经不通，
   * 主动 close() 触发 onclose → 重连，避免半开连接长期无感知。
   */
  private startLivenessCheck(): void {
    this.stopLivenessCheck()
    this.livenessTimer = setInterval(() => {
      if (this.closed || !this.ws) return
      if (Date.now() - this.lastActivityAt > LIVENESS_TIMEOUT) {
        // 关闭后由 onclose 走正常重连流程
        this.ws.close()
      }
    }, LIVENESS_CHECK_INTERVAL)
  }

  private stopLivenessCheck(): void {
    if (this.livenessTimer) {
      clearInterval(this.livenessTimer)
      this.livenessTimer = null
    }
  }
}
