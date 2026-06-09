import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useChatStore = defineStore('chat', () => {
  const selectedPeerRef = ref<string | null>(null)
  const draftByPeerRef = ref<Record<string, string>>({})
  const userScrolledUp = ref(false)

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

  return { selectedPeerRef, draftByPeerRef, userScrolledUp, selectPeer, saveDraft, getDraft }
})
