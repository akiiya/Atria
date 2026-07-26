import { ref } from 'vue'
// en 作为 fallback 必须同步可用，因此静态引入。
// 其余语言按需动态加载，避免所有用户都下载全部 10 份语言包。
import { en } from './locales/en'

export type Locale = 'zh-CN' | 'zh-TW' | 'en' | 'ja' | 'ko' | 'de' | 'fr' | 'es' | 'pt-BR' | 'ru'

export interface LocaleInfo {
  code: Locale
  label: string
}

export const locales: LocaleInfo[] = [
  { code: 'zh-CN', label: '简体中文' },
  { code: 'zh-TW', label: '繁體中文' },
  { code: 'en', label: 'English' },
  { code: 'ja', label: '日本語' },
  { code: 'ko', label: '한국어' },
  { code: 'de', label: 'Deutsch' },
  { code: 'fr', label: 'Français' },
  { code: 'es', label: 'Español' },
  { code: 'pt-BR', label: 'Português' },
  { code: 'ru', label: 'Русский' },
]

const SUPPORTED: readonly Locale[] = locales.map(l => l.code)

/** 各语言包的动态加载器。Vite 会为每个语言生成独立 chunk。 */
const loaders: Record<Exclude<Locale, 'en'>, () => Promise<Record<string, string>>> = {
  'zh-CN': () => import('./locales/zh-CN').then(m => m.zhCN),
  'zh-TW': () => import('./locales/zh-TW').then(m => m.zhTW),
  'ja': () => import('./locales/ja').then(m => m.ja),
  'ko': () => import('./locales/ko').then(m => m.ko),
  'de': () => import('./locales/de').then(m => m.de),
  'fr': () => import('./locales/fr').then(m => m.fr),
  'es': () => import('./locales/es').then(m => m.es),
  'pt-BR': () => import('./locales/pt-BR').then(m => m.ptBR),
  'ru': () => import('./locales/ru').then(m => m.ru),
}

// 已加载的语言包。en 始终可用。
const messages = ref<Partial<Record<Locale, Record<string, string>>>>({ en })

const STORAGE_KEY = 'atria_locale'

function isSupported(code: string): code is Locale {
  return (SUPPORTED as readonly string[]).includes(code)
}

export function detectLocale(): Locale {
  // 1. localStorage 中用户显式选择过的语言
  const stored = typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) : null
  if (stored && isSupported(stored)) return stored

  // 2. 浏览器语言偏好
  if (typeof navigator !== 'undefined') {
    const browserLangs = navigator.languages || [navigator.language]
    for (const lang of browserLangs) {
      if (!lang) continue
      // 精确匹配
      if (isSupported(lang)) return lang
      // 前缀匹配（zh-HK → zh-CN，pt → pt-BR）
      const prefix = lang.split('-')[0]
      const match = SUPPORTED.find(k => k === prefix || k.startsWith(prefix + '-'))
      if (match) return match
    }
  }

  // 3. 兜底
  return 'en'
}

const currentLocale = ref<Locale>(detectLocale())

/**
 * 加载指定语言包。已加载过则直接返回。
 * en 已静态引入，无需加载。
 */
export async function loadLocaleMessages(locale: Locale): Promise<void> {
  if (locale === 'en' || messages.value[locale]) return
  try {
    const loaded = await loaders[locale]()
    messages.value = { ...messages.value, [locale]: loaded }
  } catch {
    // 加载失败时保持 en fallback，不阻断应用
  }
}

/**
 * 应用启动时调用：加载当前检测到的语言，并设置 document.lang。
 * 必须在 mount 之前 await，否则首屏会短暂显示英文。
 */
export async function initI18n(): Promise<void> {
  await loadLocaleMessages(currentLocale.value)
  if (typeof document !== 'undefined') {
    document.documentElement.lang = currentLocale.value
  }
}

export function useI18n() {
  function t(key: string): string {
    return messages.value[currentLocale.value]?.[key] || messages.value.en?.[key] || key
  }

  async function setLocale(locale: Locale) {
    await loadLocaleMessages(locale)
    currentLocale.value = locale
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(STORAGE_KEY, locale)
    }
    if (typeof document !== 'undefined') {
      document.documentElement.lang = locale
    }
  }

  return {
    locale: currentLocale,
    t,
    setLocale,
    locales,
  }
}
