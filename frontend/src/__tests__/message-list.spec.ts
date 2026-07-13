import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MessageList from '@/features/chat/MessageList.vue'
import type { ChatMessage, PeerType } from '@/types/chat'

function makeMessage(overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id: 1,
    peer_ref: 'u_123',
    direction: 'out' as const,
    sender_name: 'Test User',
    text: 'Hello',
    sent_at: '2025-06-29T10:00:00Z',
    is_outgoing: true,
    status: 'sent' as const,
    message_type: 'text' as const,
    ...overrides,
  }
}

function mountList(props: Partial<{
  messages: ChatMessage[]
  hasOlder: boolean
  loadingOlder: boolean
  olderError: string | null
  peerType: PeerType
  peerRef: string
}> = {}) {
  return mount(MessageList, {
    props: {
      messages: [],
      hasOlder: false,
      loadingOlder: false,
      olderError: null,
      ...props,
    },
    global: {
      stubs: {
        MessageBubble: { template: '<div class="stub-bubble">{{ message.text }}</div>', props: ['message', 'peerType'] },
        ServiceMessage: { template: '<div class="stub-service" />', props: ['message'] },
        DateDivider: { template: '<div class="stub-divider" />', props: ['date'] },
      },
    },
  })
}

describe('MessageList', () => {
  it('renders messages', () => {
    const messages = [
      makeMessage({ id: 1, text: 'First message' }),
      makeMessage({ id: 2, text: 'Second message', sent_at: '2025-06-29T10:01:00Z' }),
    ]
    const wrapper = mountList({ messages })
    const bubbles = wrapper.findAll('.stub-bubble')
    expect(bubbles.length).toBe(2)
  })

  it('shows empty state when no messages', () => {
    const wrapper = mountList({ messages: [] })
    expect(wrapper.find('.message-empty').exists()).toBe(true)
  })

  it('hides empty state when messages exist', () => {
    const wrapper = mountList({ messages: [makeMessage()] })
    expect(wrapper.find('.message-empty').exists()).toBe(false)
  })

  it('shows older-end indicator when hasOlder is false and messages exist', () => {
    const wrapper = mountList({ messages: [makeMessage()], hasOlder: false })
    expect(wrapper.find('.older-end').exists()).toBe(true)
  })

  it('hides older-end indicator when hasOlder is true', () => {
    const wrapper = mountList({ messages: [makeMessage()], hasOlder: true })
    expect(wrapper.find('.older-end').exists()).toBe(false)
  })

  it('shows loading spinner when loadingOlder is true', () => {
    const wrapper = mountList({ messages: [makeMessage()], loadingOlder: true })
    expect(wrapper.find('.older-loading').exists()).toBe(true)
    expect(wrapper.find('.older-loading-spinner').exists()).toBe(true)
  })

  it('shows error message when olderError is provided', () => {
    const wrapper = mountList({ messages: [makeMessage()], olderError: 'Load failed' })
    expect(wrapper.find('.older-error').exists()).toBe(true)
    expect(wrapper.find('.older-error').text()).toBe('Load failed')
  })

  it('renders service messages with ServiceMessage component', () => {
    const messages = [
      makeMessage({ id: 1, message_type: 'service', text: 'User joined' }),
    ]
    const wrapper = mountList({ messages })
    expect(wrapper.find('.stub-service').exists()).toBe(true)
  })

  it('renders text messages with MessageBubble component', () => {
    const messages = [
      makeMessage({ id: 1, message_type: 'text', text: 'Hello' }),
    ]
    const wrapper = mountList({ messages })
    expect(wrapper.find('.stub-bubble').exists()).toBe(true)
  })

  it('renders date dividers for different days', () => {
    const messages = [
      makeMessage({ id: 1, sent_at: '2025-06-28T10:00:00Z' }),
      makeMessage({ id: 2, sent_at: '2025-06-29T10:00:00Z' }),
    ]
    const wrapper = mountList({ messages })
    // Two different days should produce two date dividers
    const dividers = wrapper.findAll('.stub-divider')
    expect(dividers.length).toBe(2)
  })

  it('renders a single date divider for same-day messages', () => {
    const messages = [
      makeMessage({ id: 1, sent_at: '2025-06-29T10:00:00Z' }),
      makeMessage({ id: 2, sent_at: '2025-06-29T11:00:00Z' }),
    ]
    const wrapper = mountList({ messages })
    const dividers = wrapper.findAll('.stub-divider')
    expect(dividers.length).toBe(1)
  })
})
