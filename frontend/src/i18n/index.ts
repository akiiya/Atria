import { ref } from 'vue'
import { zhCN } from './locales/zh-CN'
import { zhTW } from './locales/zh-TW'
import { en } from './locales/en'
import { ja } from './locales/ja'
import { ko } from './locales/ko'
import { de } from './locales/de'
import { fr } from './locales/fr'
import { es } from './locales/es'
import { ptBR } from './locales/pt-BR'
import { ru } from './locales/ru'

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

const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': zhCN,
  'zh-TW': zhTW,
  'en': en,
  'ja': ja,
  'ko': ko,
  'de': de,
  'fr': fr,
  'es': es,
  'pt-BR': ptBR,
  'ru': ru,
}

const STORAGE_KEY = 'atria_locale'

function detectLocale(): Locale {
  // 1. Check localStorage
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored && stored in messages) return stored as Locale

  // 2. Check browser languages
  const browserLangs = navigator.languages || [navigator.language]
  for (const lang of browserLangs) {
    // Exact match
    if (lang in messages) return lang as Locale
    // Prefix match (e.g., zh-HK → zh-TW, pt → pt-BR)
    const prefix = lang.split('-')[0]
    const match = Object.keys(messages).find(k => k === prefix || k.startsWith(prefix + '-'))
    if (match) return match as Locale
  }

  // 3. Fallback
  return 'en'
}

const currentLocale = ref<Locale>(detectLocale())

export function useI18n() {
  function t(key: string): string {
    return messages[currentLocale.value]?.[key] || messages['en']?.[key] || key
  }

  function setLocale(locale: Locale) {
    currentLocale.value = locale
    localStorage.setItem(STORAGE_KEY, locale)
    document.documentElement.lang = locale
  }

  return {
    locale: currentLocale,
    t,
    setLocale,
    locales,
  }
}
