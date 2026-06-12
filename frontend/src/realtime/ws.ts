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

export class RealtimeClient {
  private ws: WebSocket | null = null
  private state: WSState = 'disconnected'
  private reconnectDelay = INITIAL_RECONNECT_DELAY
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
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
    }

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as RealtimeEvent
        this.options.onEvent(data)
      } catch {
        // 忽略无法解析的消息
      }
    }

    this.ws.onclose = () => {
      this.ws = null
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

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, this.reconnectDelay)

    // 指数退避
    this.reconnectDelay = Math.min(
      this.reconnectDelay * RECONNECT_BACKOFF_FACTOR,
      MAX_RECONNECT_DELAY
    )
  }
}
