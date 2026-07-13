import { useI18n } from '@/i18n'

export function usePeerType() {
  const { t } = useI18n()

  function peerTypeLabel(type: string | undefined): string {
    switch (type) {
      case 'user': return ''
      case 'bot': return t('peerType.bot')
      case 'chat': return t('peerType.group')
      case 'supergroup': return t('peerType.supergroup')
      case 'channel': return t('peerType.channel')
      default: return ''
    }
  }

  function peerTypeIcon(type: string | undefined): string {
    switch (type) {
      case 'bot': return '\u{1F916}'
      case 'chat': return '\u{1F465}'
      case 'supergroup': return '\u{1F465}'
      case 'channel': return '\u{1F4E2}'
      default: return ''
    }
  }

  return { peerTypeLabel, peerTypeIcon }
}
