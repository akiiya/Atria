import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import MediaMessage from '@/features/chat/MediaMessage.vue'
import type { ChatMessage, MediaInfo } from '@/types/chat'

vi.mock('@/api/media', () => ({
  getMediaStatus: vi.fn(),
  downloadMedia: vi.fn(),
  getMediaContentUrl: (id: number, peerRef: string) =>
    `/api/media/${id}/content?peer_ref=${encodeURIComponent(peerRef)}`,
}))

import { getMediaStatus, downloadMedia } from '@/api/media'
const mockGetStatus = vi.mocked(getMediaStatus)
const mockDownload = vi.mocked(downloadMedia)

function makeMessage(overrides: Partial<ChatMessage> = {}, media?: MediaInfo): ChatMessage {
  return {
    id: 1,
    telegram_message_id: 1,
    peer_ref: 'u_123',
    direction: 'in',
    sender_name: 'Sender',
    text: '',
    sent_at: '2026-07-01T10:00:00Z',
    is_outgoing: false,
    status: 'sent',
    message_type: 'photo',
    media,
    ...overrides,
  }
}

function mountMedia(message: ChatMessage) {
  // 每个测试独立的 QueryClient，避免用例之间共享缓存
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return mount(MediaMessage, {
    props: { message },
    global: {
      plugins: [[VueQueryPlugin, { queryClient }]],
      stubs: { ImageLightbox: true },
    },
  })
}

describe('MediaMessage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetStatus.mockResolvedValue({ ok: true, status: 'none', available: false })
  })

  describe('缓存状态查询', () => {
    it('媒体类型消息会查询缓存状态', async () => {
      mountMedia(makeMessage())
      await vi.waitFor(() => expect(mockGetStatus).toHaveBeenCalledWith(1, 'u_123'))
    })

    it('非媒体类型不查询缓存状态', async () => {
      mountMedia(makeMessage({ message_type: 'text' }))
      // 给查询一个可能触发的时间窗口
      await new Promise((r) => setTimeout(r, 20))
      expect(mockGetStatus).not.toHaveBeenCalled()
    })

    it('已缓存时渲染图片而非占位框', async () => {
      mockGetStatus.mockResolvedValue({ ok: true, status: 'cached', available: true })
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('img.media-img').exists()).toBe(true))
      expect(w.find('.media-placeholder').exists()).toBe(false)
    })

    it('未缓存时渲染占位框', async () => {
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('.media-placeholder').exists()).toBe(true))
      expect(w.find('img.media-img').exists()).toBe(false)
    })
  })

  describe('图片布局稳定性', () => {
    it('图片设置 loading=lazy 和 decoding=async', async () => {
      mockGetStatus.mockResolvedValue({ ok: true, status: 'cached', available: true })
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('img.media-img').exists()).toBe(true))

      const img = w.find('img.media-img')
      expect(img.attributes('loading')).toBe('lazy')
      expect(img.attributes('decoding')).toBe('async')
    })

    it('有宽高时图片带 width/height 属性以预留空间', async () => {
      mockGetStatus.mockResolvedValue({ ok: true, status: 'cached', available: true })
      const w = mountMedia(makeMessage({}, { width: 800, height: 600 }))
      await vi.waitFor(() => expect(w.find('img.media-img').exists()).toBe(true))

      const img = w.find('img.media-img')
      expect(img.attributes('width')).toBe('800')
      expect(img.attributes('height')).toBe('600')
      // aspect-ratio 保证下载完成前后尺寸一致
      expect(img.attributes('style')).toContain('aspect-ratio')
    })

    it('占位框与图片使用相同的预留尺寸', async () => {
      const w = mountMedia(makeMessage({}, { width: 800, height: 600 }))
      await vi.waitFor(() => expect(w.find('.media-placeholder').exists()).toBe(true))
      expect(w.find('.media-placeholder').attributes('style')).toContain('aspect-ratio')
    })

    it('无宽高信息时不设置内联尺寸', async () => {
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('.media-placeholder').exists()).toBe(true))
      const style = w.find('.media-placeholder').attributes('style')
      expect(style === undefined || !style.includes('aspect-ratio')).toBe(true)
    })
  })

  describe('下载', () => {
    it('下载成功后切换为已缓存并显示图片', async () => {
      mockDownload.mockResolvedValue({ ok: true, status: 'cached' })
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('.media-placeholder').exists()).toBe(true))

      await w.find('.media-placeholder').trigger('click')
      await vi.waitFor(() => expect(w.find('img.media-img').exists()).toBe(true))
      expect(mockDownload).toHaveBeenCalledWith(1, 'u_123')
    })

    it('下载失败时显示错误信息', async () => {
      mockDownload.mockResolvedValue({ ok: false, message: '文件过大' })
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('.media-placeholder').exists()).toBe(true))

      await w.find('.media-placeholder').trigger('click')
      await vi.waitFor(() => expect(w.find('.media-error').exists()).toBe(true))
      expect(w.find('.media-error').text()).toContain('文件过大')
    })

    it('下载抛异常时显示错误信息', async () => {
      mockDownload.mockRejectedValue(new Error('网络中断'))
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('.media-placeholder').exists()).toBe(true))

      await w.find('.media-placeholder').trigger('click')
      await vi.waitFor(() => expect(w.find('.media-error').exists()).toBe(true))
      expect(w.find('.media-error').text()).toContain('网络中断')
    })
  })

  describe('Lightbox 挂载', () => {
    it('未打开时不挂载 lightbox（避免每条消息常驻实例）', async () => {
      mockGetStatus.mockResolvedValue({ ok: true, status: 'cached', available: true })
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('img.media-img').exists()).toBe(true))
      expect(w.findComponent({ name: 'ImageLightbox' }).exists()).toBe(false)
    })

    it('点击已缓存图片后挂载 lightbox', async () => {
      mockGetStatus.mockResolvedValue({ ok: true, status: 'cached', available: true })
      const w = mountMedia(makeMessage())
      await vi.waitFor(() => expect(w.find('img.media-img').exists()).toBe(true))

      await w.find('.media-preview').trigger('click')
      await vi.waitFor(() =>
        expect(w.findComponent({ name: 'ImageLightbox' }).exists()).toBe(true)
      )
    })
  })

  describe('视频', () => {
    it('已缓存视频使用 preload=metadata 避免预加载整个文件', async () => {
      mockGetStatus.mockResolvedValue({ ok: true, status: 'cached', available: true })
      const w = mountMedia(makeMessage({ message_type: 'video' }))
      await vi.waitFor(() => expect(w.find('video').exists()).toBe(true))

      const video = w.find('video')
      expect(video.attributes('preload')).toBe('metadata')
      expect(video.attributes('playsinline')).toBeDefined()
    })
  })
})
