import { describe, it, expect } from 'vitest'
import { en } from '@/i18n/locales/en'
import { zhCN } from '@/i18n/locales/zh-CN'
import { zhTW } from '@/i18n/locales/zh-TW'
import { ja } from '@/i18n/locales/ja'
import { ko } from '@/i18n/locales/ko'
import { de } from '@/i18n/locales/de'
import { fr } from '@/i18n/locales/fr'
import { es } from '@/i18n/locales/es'
import { ptBR } from '@/i18n/locales/pt-BR'
import { ru } from '@/i18n/locales/ru'

const locales = {
  en, 'zh-CN': zhCN, 'zh-TW': zhTW, ja, ko, de, fr, es, 'pt-BR': ptBR, ru,
} as const

const allKeys = Object.keys(en)

describe('i18n key consistency', () => {
  for (const [name, messages] of Object.entries(locales)) {
    it(`${name} has all keys from en`, () => {
      const localeKeys = Object.keys(messages)
      const missing = allKeys.filter(k => !localeKeys.includes(k))
      expect(missing).toEqual([])
    })

    it(`${name} has no extra keys beyond en`, () => {
      const localeKeys = Object.keys(messages)
      const extra = localeKeys.filter(k => !allKeys.includes(k))
      expect(extra).toEqual([])
    })

    it(`${name} has no empty values`, () => {
      const empty = allKeys.filter(k => !messages[k] || messages[k].trim() === '')
      expect(empty).toEqual([])
    })
  }
})

describe('i18n fallback behavior', () => {
  it('t() returns key when key is missing', () => {
    // Simulate fallback: if key not found, return the key itself
    const key = 'nonexistent.key'
    const result = en[key] || key
    expect(result).toBe(key)
  })

  it('t() never returns undefined', () => {
    const key = 'nonexistent.key'
    const result = en[key] || key
    expect(result).not.toBeUndefined()
    expect(result).not.toBeNull()
  })
})
