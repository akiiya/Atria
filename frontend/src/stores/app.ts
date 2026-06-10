import { defineStore } from 'pinia'
import { ref } from 'vue'

const THEME_KEY = 'atria-theme'

function resolveSystemTheme(): 'light' | 'dark' {
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return 'dark' // default to dark to match legacy behavior
}

function resolveTheme(theme: 'light' | 'dark' | 'system'): 'light' | 'dark' {
  return theme === 'system' ? resolveSystemTheme() : theme
}

export const useAppStore = defineStore('app', () => {
  // Read from legacy localStorage key, default to 'system'
  const storedTheme = (typeof localStorage !== 'undefined' ? localStorage.getItem(THEME_KEY) : null) as 'light' | 'dark' | 'system' | null
  const theme = ref<'light' | 'dark' | 'system'>(storedTheme || 'system')
  const sidebarCollapsed = ref(false)
  const mobilePanelMode = ref<'list' | 'chat'>('list')

  function setTheme(t: 'light' | 'dark' | 'system') {
    theme.value = t
    // Save to legacy localStorage key
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(THEME_KEY, t)
    }
    // Apply resolved theme to document root
    document.documentElement.setAttribute('data-theme', resolveTheme(t))
  }

  // Apply theme on init
  setTheme(theme.value)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return { theme, sidebarCollapsed, mobilePanelMode, setTheme, toggleSidebar }
})
