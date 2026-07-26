import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import MessageComposer from '@/features/chat/MessageComposer.vue'
import { useChatStore } from '@/stores/chat'

vi.mock('@/api/chat', () => ({
  sendMessage: vi.fn(),
}))

import { sendMessage } from '@/api/chat'
const mockSend = vi.mocked(sendMessage)

function mountComposer(peerRef = 'u_123', accountId = 1) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return mount(MessageComposer, {
    props: { peerRef, accountId },
    global: {
      plugins: [[VueQueryPlugin, { queryClient }]],
    },
  })
}

describe('MessageComposer', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockSend.mockResolvedValue({ ok: true, message: undefined })
  })

  describe('草稿持久化', () => {
    it('挂载时恢复该会话已保存的草稿', () => {
      const chat = useChatStore()
      chat.saveDraft('u_123', '未发送的内容')

      const w = mountComposer('u_123')
      expect((w.find('textarea').element as HTMLTextAreaElement).value).toBe('未发送的内容')
    })

    it('无草稿时输入框为空', () => {
      const w = mountComposer('u_123')
      expect((w.find('textarea').element as HTMLTextAreaElement).value).toBe('')
    })

    it('切换会话时保存旧会话草稿', async () => {
      const chat = useChatStore()
      const w = mountComposer('u_123')

      await w.find('textarea').setValue('给 A 的草稿')
      await w.setProps({ peerRef: 'u_456' })

      expect(chat.getDraft('u_123')).toBe('给 A 的草稿')
    })

    it('切换会话时载入新会话草稿', async () => {
      const chat = useChatStore()
      chat.saveDraft('u_456', '给 B 的草稿')

      const w = mountComposer('u_123')
      await w.find('textarea').setValue('给 A 的草稿')
      await w.setProps({ peerRef: 'u_456' })

      expect((w.find('textarea').element as HTMLTextAreaElement).value).toBe('给 B 的草稿')
    })

    it('切回原会话时草稿仍然存在', async () => {
      const w = mountComposer('u_123')

      await w.find('textarea').setValue('保留我')
      await w.setProps({ peerRef: 'u_456' })
      await w.setProps({ peerRef: 'u_123' })

      expect((w.find('textarea').element as HTMLTextAreaElement).value).toBe('保留我')
    })

    it('卸载时保存草稿', () => {
      const chat = useChatStore()
      const w = mountComposer('u_123')

      w.find('textarea').setValue('离开前的内容')
      w.unmount()

      expect(chat.getDraft('u_123')).toBe('离开前的内容')
    })

    it('发送成功后清除草稿', async () => {
      const chat = useChatStore()
      chat.saveDraft('u_123', '旧草稿')

      const w = mountComposer('u_123')
      await w.find('textarea').setValue('要发送的消息')
      await w.find('button').trigger('click')

      await vi.waitFor(() => expect(chat.getDraft('u_123')).toBe(''))
    })

    it('切换会话时清空错误提示', async () => {
      const w = mountComposer('u_123')

      // 超长文本触发本地校验错误
      await w.find('textarea').setValue('x'.repeat(5000))
      await w.find('button').trigger('click')
      await vi.waitFor(() => expect(w.find('.composer-error').exists()).toBe(true))

      await w.setProps({ peerRef: 'u_456' })
      expect(w.find('.composer-error').exists()).toBe(false)
    })
  })

  describe('发送', () => {
    it('内容为空时发送按钮禁用', () => {
      const w = mountComposer()
      expect(w.find('button').attributes('disabled')).toBeDefined()
    })

    it('有内容时发送按钮可用', async () => {
      const w = mountComposer()
      await w.find('textarea').setValue('你好')
      expect(w.find('button').attributes('disabled')).toBeUndefined()
    })

    it('仅空白内容不触发发送', async () => {
      const w = mountComposer()
      await w.find('textarea').setValue('   ')
      await w.find('button').trigger('click')
      expect(mockSend).not.toHaveBeenCalled()
    })

    it('超过 4096 字符时拒绝发送并提示', async () => {
      const w = mountComposer()
      await w.find('textarea').setValue('x'.repeat(5000))
      await w.find('button').trigger('click')

      expect(mockSend).not.toHaveBeenCalled()
      expect(w.find('.composer-error').exists()).toBe(true)
    })

    it('Enter 触发发送', async () => {
      const w = mountComposer()
      await w.find('textarea').setValue('你好')
      await w.find('textarea').trigger('keydown', { key: 'Enter' })

      await vi.waitFor(() => expect(mockSend).toHaveBeenCalled())
    })

    it('Shift+Enter 不触发发送（换行）', async () => {
      const w = mountComposer()
      await w.find('textarea').setValue('你好')
      await w.find('textarea').trigger('keydown', { key: 'Enter', shiftKey: true })

      expect(mockSend).not.toHaveBeenCalled()
    })

    it('发送时去除首尾空白', async () => {
      const w = mountComposer('u_123')
      await w.find('textarea').setValue('  你好  ')
      await w.find('button').trigger('click')

      await vi.waitFor(() =>
        expect(mockSend).toHaveBeenCalledWith('u_123', '你好', expect.any(String))
      )
    })

    it('服务端返回失败时显示错误', async () => {
      mockSend.mockResolvedValue({ ok: false, error: '会话不可用' })
      const w = mountComposer()

      await w.find('textarea').setValue('你好')
      await w.find('button').trigger('click')

      await vi.waitFor(() => expect(w.find('.composer-error').text()).toContain('会话不可用'))
    })

    it('网络异常时显示错误', async () => {
      mockSend.mockRejectedValue(new Error('网络中断'))
      const w = mountComposer()

      await w.find('textarea').setValue('你好')
      await w.find('button').trigger('click')

      await vi.waitFor(() => expect(w.find('.composer-error').text()).toContain('网络中断'))
    })
  })
})
