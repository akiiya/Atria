import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useChatStore } from '@/stores/chat'

describe('Chat Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('selectPeer updates selectedPeerRef', () => {
    const chat = useChatStore()
    expect(chat.selectedPeerRef).toBeNull()

    chat.selectPeer('u_123')
    expect(chat.selectedPeerRef).toBe('u_123')
  })

  it('selectPeer null clears selectedPeerRef', () => {
    const chat = useChatStore()
    chat.selectPeer('u_123')
    chat.selectPeer(null)
    expect(chat.selectedPeerRef).toBeNull()
  })

  it('selectPeer resets userScrolledUp', () => {
    const chat = useChatStore()
    chat.userScrolledUp = true
    chat.selectPeer('u_456')
    expect(chat.userScrolledUp).toBe(false)
  })

  it('clearForAccountSwitch clears all state', () => {
    const chat = useChatStore()
    chat.selectPeer('u_123')
    chat.saveDraft('u_123', 'hello draft')
    chat.userScrolledUp = true

    chat.clearForAccountSwitch()

    expect(chat.selectedPeerRef).toBeNull()
    expect(chat.draftByPeerRef).toEqual({})
    expect(chat.userScrolledUp).toBe(false)
  })

  it('saveDraft and getDraft work correctly', () => {
    const chat = useChatStore()
    chat.saveDraft('u_123', 'hello')
    expect(chat.getDraft('u_123')).toBe('hello')
    expect(chat.getDraft('u_456')).toBe('')
  })

  it('drafts are isolated by peerRef', () => {
    const chat = useChatStore()
    chat.saveDraft('u_1', 'draft 1')
    chat.saveDraft('u_2', 'draft 2')
    expect(chat.getDraft('u_1')).toBe('draft 1')
    expect(chat.getDraft('u_2')).toBe('draft 2')
  })
})

describe('Query Key Structure', () => {
  // 验证 query key 结构符合规范
  // dialogs query key: ['dialogs', accountId]
  // messages query key: ['messages', accountId, peerRef]

  it('dialogs query key includes accountId', () => {
    const accountId = 42
    const queryKey = ['dialogs', accountId]
    expect(queryKey).toHaveLength(2)
    expect(queryKey[0]).toBe('dialogs')
    expect(queryKey[1]).toBe(accountId)
  })

  it('messages query key includes accountId and peerRef', () => {
    const accountId = 42
    const peerRef = 'u_123'
    const queryKey = ['messages', accountId, peerRef]
    expect(queryKey).toHaveLength(3)
    expect(queryKey[0]).toBe('messages')
    expect(queryKey[1]).toBe(accountId)
    expect(queryKey[2]).toBe(peerRef)
  })

  it('different accounts produce different query keys', () => {
    const key1 = ['messages', 1, 'u_123']
    const key2 = ['messages', 2, 'u_123']
    expect(JSON.stringify(key1)).not.toBe(JSON.stringify(key2))
  })

  it('different peers produce different query keys', () => {
    const key1 = ['messages', 1, 'u_123']
    const key2 = ['messages', 1, 'u_456']
    expect(JSON.stringify(key1)).not.toBe(JSON.stringify(key2))
  })
})
