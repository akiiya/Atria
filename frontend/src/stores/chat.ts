import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useChatStore = defineStore('chat', () => {
  const selectedPeerRef = ref<string | null>(null)
  const draftByPeerRef = ref<Record<string, string>>({})
  const userScrolledUp = ref(false)

  // peer stale 追踪：标记需要在切换时 force refresh 的 peer
  const stalePeers = ref(new Set<string>())

  function selectPeer(ref: string | null) {
    selectedPeerRef.value = ref
    userScrolledUp.value = false
  }

  function saveDraft(peerRef: string, text: string) {
    draftByPeerRef.value[peerRef] = text
  }

  function getDraft(peerRef: string): string {
    return draftByPeerRef.value[peerRef] || ''
  }

  /** 标记 peer 为 stale，切换到该 peer 时需要 force refresh */
  function markPeerStale(peerRef: string) {
    stalePeers.value.add(peerRef)
  }

  /** 清除 peer 的 stale 标记 */
  function clearPeerStale(peerRef: string) {
    stalePeers.value.delete(peerRef)
  }

  /** 检查 peer 是否 stale */
  function isPeerStale(peerRef: string): boolean {
    return stalePeers.value.has(peerRef)
  }

  /** 切换账号时清理聊天状态，防止显示旧账号数据 */
  function clearForAccountSwitch() {
    selectedPeerRef.value = null
    draftByPeerRef.value = {}
    userScrolledUp.value = false
    stalePeers.value.clear()
  }

  return {
    selectedPeerRef, draftByPeerRef, userScrolledUp, stalePeers,
    selectPeer, saveDraft, getDraft,
    markPeerStale, clearPeerStale, isPeerStale,
    clearForAccountSwitch,
  }
})
