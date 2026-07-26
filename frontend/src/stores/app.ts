import { defineStore } from 'pinia'
import { ref } from 'vue'

const THEME_KEY = 'atria-theme'

type Theme = 'light' | 'dark' | 'system'

function systemMediaQuery(): MediaQueryList | null {
  if (typeof window === 'undefined' || !window.matchMedia) return null
  return window.matchMedia('(prefers-color-scheme: dark)')
}

function resolveSystemTheme(): 'light' | 'dark' {
  const mq = systemMediaQuery()
  if (!mq) return 'light'
  return mq.matches ? 'dark' : 'light'
}

function resolveTheme(theme: Theme): 'light' | 'dark' {
  return theme === 'system' ? resolveSystemTheme() : theme
}

export const useAppStore = defineStore('app', () => {
  const storedTheme = (typeof localStorage !== 'undefined' ? localStorage.getItem(THEME_KEY) : null) as Theme | null
  const theme = ref<Theme>(storedTheme || 'system')
  const sidebarCollapsed = ref(false)
  const mobilePanelMode = ref<'list' | 'chat'>('list')

  function applyTheme(t: Theme) {
    document.documentElement.setAttribute('data-theme', resolveTheme(t))
  }

  function setTheme(t: Theme) {
    theme.value = t
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(THEME_KEY, t)
    }
    applyTheme(t)
  }

  // 应用初始主题。index.html 中的 pre-paint 脚本已设置过一次，
  // 这里保证 store 状态与 DOM 一致。
  applyTheme(theme.value)

  // 跟随系统模式时，监听系统主题实时变化
  const mq = systemMediaQuery()
  if (mq) {
    const onSystemThemeChange = () => {
      if (theme.value === 'system') applyTheme('system')
    }
    if (typeof mq.addEventListener === 'function') {
      mq.addEventListener('change', onSystemThemeChange)
    } else if (typeof mq.addListener === 'function') {
      // Safari < 14
      mq.addListener(onSystemThemeChange)
    }
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return { theme, sidebarCollapsed, mobilePanelMode, setTheme, toggleSidebar }
})
