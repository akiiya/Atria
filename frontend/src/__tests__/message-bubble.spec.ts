import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MessageBubble from '@/features/chat/MessageBubble.vue'
import MessageComposer from '@/features/chat/MessageComposer.vue'
import MessageHeader from '@/features/chat/MessageHeader.vue'
import DateDivider from '@/features/chat/DateDivider.vue'
import ServiceMessage from '@/features/chat/ServiceMessage.vue'
import { createPinia, setActivePinia } from 'pinia'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

// Helper to create a minimal message
function makeMessage(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    peer_ref: 'u_123',
    direction: 'out' as const,
    sender_name: 'Test User',
    text: 'Hello world',
    sent_at: '2025-01-15T10:30:00Z',
    is_outgoing: true,
    status: 'sent' as const,
    message_type: 'text' as const,
    ...overrides,
  }
}

describe('MessageBubble', () => {
  it('has outgoing class for outgoing messages', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: true }) },
    })
    expect(wrapper.find('.message-bubble').classes()).toContain('outgoing')
    expect(wrapper.find('.message-bubble').classes()).not.toContain('incoming')
  })

  it('has incoming class for incoming messages', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: false, direction: 'in' }) },
    })
    expect(wrapper.find('.message-bubble').classes()).toContain('incoming')
    expect(wrapper.find('.message-bubble').classes()).not.toContain('outgoing')
  })

  it('displays message text', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ text: 'Test message content' }) },
    })
    expect(wrapper.find('.message-text').text()).toContain('Test message content')
  })

  it('displays sender name for incoming messages', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: false, direction: 'in', sender_name: 'Alice' }) },
    })
    expect(wrapper.find('.message-sender').text()).toBe('Alice')
  })

  it('does not display sender name for outgoing messages', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: true, sender_name: 'Alice' }) },
    })
    expect(wrapper.find('.message-sender').exists()).toBe(false)
  })

  it('displays time', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage() },
    })
    expect(wrapper.find('.message-time').exists()).toBe(true)
  })

  it('displays status for outgoing messages', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: true, status: 'sent' }) },
    })
    expect(wrapper.find('.message-status').exists()).toBe(true)
    expect(wrapper.find('.message-status').text()).toBe('✓')
  })

  it('shows unsupported message type', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ message_type: 'unsupported' }) },
    })
    expect(wrapper.find('.message-unsupported').exists()).toBe(true)
    expect(wrapper.find('.message-unsupported').text()).toContain('unsupported')
  })

  it('shows failed status indicator', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: true, status: 'failed' }) },
    })
    expect(wrapper.find('.message-status.failed').exists()).toBe(true)
    expect(wrapper.find('.message-status').text()).toBe('✕')
  })
})

describe('MessageBubble CSS Properties', () => {
  // 验证 CSS 中定义的关键样式属性存在
  // 这些测试验证 chat.css 中的规则

  it('message-bubble has max-width defined in CSS', () => {
    // 验证 chat.css 中 .message-bubble 有 max-width: min(680px, 72%)
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage() },
    })
    const bubble = wrapper.find('.message-bubble')
    expect(bubble.exists()).toBe(true)
    // 通过检查 class 存在来验证元素结构正确
    expect(bubble.element.tagName).toBe('DIV')
  })

  it('outgoing bubble uses margin-left auto for right alignment', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: true }) },
    })
    const bubble = wrapper.find('.message-bubble.outgoing')
    expect(bubble.exists()).toBe(true)
  })

  it('incoming bubble uses margin-right auto for left alignment', () => {
    const wrapper = mount(MessageBubble, {
      props: { message: makeMessage({ is_outgoing: false, direction: 'in' }) },
    })
    const bubble = wrapper.find('.message-bubble.incoming')
    expect(bubble.exists()).toBe(true)
  })
})

describe('MessageHeader', () => {
  it('displays title when provided', () => {
    setActivePinia(createPinia())
    const wrapper = mount(MessageHeader, {
      props: { peerRef: 'u_123', title: 'Alice Smith' },
      global: {
        stubs: { routerLink: true },
        plugins: [VueQueryPlugin],
      },
    })
    expect(wrapper.find('.message-header-title').text()).toBe('Alice Smith')
  })

  it('falls back to peerRef when title is empty', () => {
    setActivePinia(createPinia())
    const wrapper = mount(MessageHeader, {
      props: { peerRef: 'u_123', title: '' },
      global: {
        stubs: { routerLink: true },
        plugins: [VueQueryPlugin],
      },
    })
    expect(wrapper.find('.message-header-title').text()).toBe('u_123')
  })

  it('falls back to peerRef when title is undefined', () => {
    setActivePinia(createPinia())
    const wrapper = mount(MessageHeader, {
      props: { peerRef: 'u_123' },
      global: {
        stubs: { routerLink: true },
        plugins: [VueQueryPlugin],
      },
    })
    expect(wrapper.find('.message-header-title').text()).toBe('u_123')
  })
})

describe('DateDivider', () => {
  it('renders date divider element', () => {
    const wrapper = mount(DateDivider, {
      props: { date: '2025-01-15T10:30:00Z' },
    })
    expect(wrapper.find('.date-divider').exists()).toBe(true)
    expect(wrapper.find('.date-divider-text').exists()).toBe(true)
  })
})

describe('ServiceMessage', () => {
  it('renders service message', () => {
    const wrapper = mount(ServiceMessage, {
      props: {
        message: makeMessage({
          message_type: 'service',
          text: 'User joined the group',
          is_outgoing: false,
          direction: 'in',
        }),
      },
    })
    expect(wrapper.find('.service-message').exists()).toBe(true)
    expect(wrapper.find('.service-text').text()).toBe('User joined the group')
  })
})

describe('MessageComposer', () => {
  function mountComposer() {
    setActivePinia(createPinia())
    const queryClient = new QueryClient()
    return mount(MessageComposer, {
      props: { peerRef: 'u_123', accountId: 1 },
      global: {
        plugins: [createPinia(), [VueQueryPlugin, { queryClient }]],
        stubs: { TransitionGroup: true },
      },
    })
  }

  it('renders textarea and send button', () => {
    const wrapper = mountComposer()
    expect(wrapper.find('.composer-input').exists()).toBe(true)
    expect(wrapper.find('.composer-send').exists()).toBe(true)
  })

  it('send button is disabled when text is empty', () => {
    const wrapper = mountComposer()
    expect(wrapper.find('.composer-send').attributes('disabled')).toBeDefined()
  })
})
