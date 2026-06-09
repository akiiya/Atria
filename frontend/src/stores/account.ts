import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AccountInfo } from '@/types/account'

export const useAccountStore = defineStore('account', () => {
  const currentAccountId = ref<number | null>(null)
  const currentAccountDisplayName = ref('')
  const accountList = ref<AccountInfo[]>([])

  function setCurrent(acc: AccountInfo | null) {
    currentAccountId.value = acc?.id ?? null
    currentAccountDisplayName.value = acc?.display_name ?? ''
  }

  function setAccounts(accounts: AccountInfo[]) {
    accountList.value = accounts
  }

  return { currentAccountId, currentAccountDisplayName, accountList, setCurrent, setAccounts }
})
