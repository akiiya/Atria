import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DialogItem from '@/features/chat/DialogItem.vue'
import type { Dialog } from '@/types/chat'

function makeDialog(overrides: Partial<Dialog> = {}): Dialog {
  return {
    peer_ref: 'u_123',
    peer_type: 'user',
    title: 'Alice',
    unread_count: 0,
    last_message_at: '2025-06-29T10:30:00Z',
    last_message_preview: 'Hello!',
    ...overrides,
  }
}

describe('DialogItem', () => {
  it('renders title correctly', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ title: 'Bob Smith' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    expect(wrapper.find('.dialog-title').text()).toContain('Bob Smith')
  })

  it('shows unread badge when unread_count > 0', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ unread_count: 5 }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const badge = wrapper.find('.dialog-unread-badge')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('5')
  })

  it('hides unread badge when unread_count is 0', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ unread_count: 0 }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    expect(wrapper.find('.dialog-unread-badge').exists()).toBe(false)
  })

  it('shows peer type label for non-user types', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ peer_type: 'bot' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const tag = wrapper.find('.dialog-type-tag')
    expect(tag.exists()).toBe(true)
    expect(tag.text().length).toBeGreaterThan(0)
  })

  it('hides peer type label for user type', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ peer_type: 'user' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    expect(wrapper.find('.dialog-type-tag').exists()).toBe(false)
  })

  it('shows peer type icon for bot', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ peer_type: 'bot' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const icon = wrapper.find('.dialog-type-icon')
    expect(icon.exists()).toBe(true)
    expect(icon.text()).toBe('\u{1F916}')
  })

  it('shows peer type icon for chat', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ peer_type: 'chat' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const icon = wrapper.find('.dialog-type-icon')
    expect(icon.exists()).toBe(true)
    expect(icon.text()).toBe('\u{1F465}')
  })

  it('shows peer type icon for supergroup', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ peer_type: 'supergroup' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const icon = wrapper.find('.dialog-type-icon')
    expect(icon.exists()).toBe(true)
    expect(icon.text()).toBe('\u{1F465}')
  })

  it('shows peer type icon for channel', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ peer_type: 'channel' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const icon = wrapper.find('.dialog-type-icon')
    expect(icon.exists()).toBe(true)
    expect(icon.text()).toBe('\u{1F4E2}')
  })

  it('hides peer type icon for user', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ peer_type: 'user' }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    expect(wrapper.find('.dialog-type-icon').exists()).toBe(false)
  })

  it('formats time for today (shows time)', () => {
    const now = new Date()
    const todayIso = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 14, 30).toISOString()
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ last_message_at: todayIso }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const timeText = wrapper.find('.dialog-time').text()
    // Today's time should contain digits and a colon separator (e.g., "14:30" or "02:30 PM")
    expect(timeText).toMatch(/\d/)
    // Should NOT be a date format like MM/DD
    expect(timeText).not.toMatch(/^\d{2}\/\d{2}$/)
  })

  it('formats time for other days (shows date)', () => {
    const pastDate = '2024-01-15T10:30:00Z'
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ last_message_at: pastDate }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    const timeText = wrapper.find('.dialog-time').text()
    expect(timeText.length).toBeGreaterThan(0)
  })

  it('handles missing last_message_at gracefully', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog({ last_message_at: undefined }), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    expect(wrapper.find('.dialog-time').text()).toBe('')
  })

  it('applies selected class when selected is true', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog(), selected: true },
      global: { stubs: { AvatarInitials: true } },
    })
    expect(wrapper.find('.dialog-item').classes()).toContain('selected')
  })

  it('does not apply selected class when selected is false', () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog(), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    expect(wrapper.find('.dialog-item').classes()).not.toContain('selected')
  })

  it('emits click event when clicked', async () => {
    const wrapper = mount(DialogItem, {
      props: { dialog: makeDialog(), selected: false },
      global: { stubs: { AvatarInitials: true } },
    })
    await wrapper.find('.dialog-item').trigger('click')
    expect(wrapper.emitted('click')).toBeDefined()
  })
})
