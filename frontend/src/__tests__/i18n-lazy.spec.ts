import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

/**
 * 语言包按需加载测试。
 * i18n 模块持有模块级状态（当前语言、已加载语言包），
 * 因此每个用例都通过 resetModules + 动态 import 拿到全新实例。
 */

const STORAGE_KEY = 'atria_locale'

async function freshI18n() {
  vi.resetModules()
  return await import('@/i18n')
}

function stubNavigatorLanguages(langs: string[]) {
  vi.stubGlobal('navigator', { languages: langs, language: langs[0] })
}

describe('i18n 按需加载', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.unstubAllGlobals()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  describe('语言检测', () => {
    it('优先使用 localStorage 中已保存的语言', async () => {
      localStorage.setItem(STORAGE_KEY, 'ja')
      stubNavigatorLanguages(['de-DE'])

      const { detectLocale } = await freshI18n()
      expect(detectLocale()).toBe('ja')
    })

    it('忽略 localStorage 中不支持的语言值', async () => {
      localStorage.setItem(STORAGE_KEY, 'xx-YY')
      stubNavigatorLanguages(['fr-FR'])

      const { detectLocale } = await freshI18n()
      expect(detectLocale()).toBe('fr')
    })

    it('精确匹配浏览器语言', async () => {
      stubNavigatorLanguages(['zh-TW'])

      const { detectLocale } = await freshI18n()
      expect(detectLocale()).toBe('zh-TW')
    })

    it('前缀匹配浏览器语言（zh-HK → zh-CN）', async () => {
      stubNavigatorLanguages(['zh-HK'])

      const { detectLocale } = await freshI18n()
      expect(detectLocale()).toBe('zh-CN')
    })

    it('前缀匹配浏览器语言（pt → pt-BR）', async () => {
      stubNavigatorLanguages(['pt'])

      const { detectLocale } = await freshI18n()
      expect(detectLocale()).toBe('pt-BR')
    })

    it('全部不匹配时回退到 en', async () => {
      stubNavigatorLanguages(['xx-YY', 'zz'])

      const { detectLocale } = await freshI18n()
      expect(detectLocale()).toBe('en')
    })
  })

  describe('翻译加载', () => {
    it('en 无需加载即可使用（静态打包为 fallback）', async () => {
      stubNavigatorLanguages(['en-US'])

      const { useI18n } = await freshI18n()
      const { t } = useI18n()
      expect(t('nav.chats')).toBe('Chats')
    })

    it('initI18n 后可使用检测到的语言', async () => {
      stubNavigatorLanguages(['zh-CN'])

      const { initI18n, useI18n } = await freshI18n()
      await initI18n()

      const { t } = useI18n()
      expect(t('nav.chats')).toBe('聊天')
    })

    it('未加载的语言在加载完成前回退到 en', async () => {
      stubNavigatorLanguages(['en-US'])

      const { useI18n } = await freshI18n()
      const { t, locale } = useI18n()

      // 直接改语言但不加载语言包，应回退到 en 而不是返回 undefined
      locale.value = 'ja'
      expect(t('nav.chats')).toBe('Chats')
    })

    it('setLocale 会加载对应语言包并切换', async () => {
      stubNavigatorLanguages(['en-US'])

      const { useI18n } = await freshI18n()
      const { t, setLocale } = useI18n()

      expect(t('nav.chats')).toBe('Chats')
      await setLocale('ja')
      expect(t('nav.chats')).toBe('チャット')
    })

    it('setLocale 会持久化选择', async () => {
      stubNavigatorLanguages(['en-US'])

      const { useI18n } = await freshI18n()
      const { setLocale } = useI18n()

      await setLocale('ko')
      expect(localStorage.getItem(STORAGE_KEY)).toBe('ko')
    })

    it('setLocale 会更新 document.lang', async () => {
      stubNavigatorLanguages(['en-US'])

      const { useI18n } = await freshI18n()
      const { setLocale } = useI18n()

      await setLocale('ru')
      expect(document.documentElement.lang).toBe('ru')
    })

    it('重复加载同一语言不会出错', async () => {
      stubNavigatorLanguages(['en-US'])

      const { loadLocaleMessages, useI18n } = await freshI18n()
      await loadLocaleMessages('de')
      await loadLocaleMessages('de')

      const { t, setLocale } = useI18n()
      await setLocale('de')
      expect(t('nav.chats')).toBeTruthy()
      expect(t('nav.chats')).not.toBe('nav.chats')
    })

    it('缺失的 key 返回 key 本身而非 undefined', async () => {
      stubNavigatorLanguages(['en-US'])

      const { useI18n } = await freshI18n()
      const { t } = useI18n()

      expect(t('this.key.does.not.exist')).toBe('this.key.does.not.exist')
    })
  })

  describe('语言列表', () => {
    it('导出全部 10 种语言', async () => {
      const { locales } = await freshI18n()
      expect(locales).toHaveLength(10)
    })

    it('每种语言都有 code 和 label', async () => {
      const { locales } = await freshI18n()
      for (const l of locales) {
        expect(l.code).toBeTruthy()
        expect(l.label).toBeTruthy()
      }
    })
  })
})
