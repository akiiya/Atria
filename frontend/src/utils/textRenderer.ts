import type { MessageEntity } from '@/types/chat'

/**
 * 将 Telegram 消息实体转换为 HTML。
 *
 * 实体按 offset 排序后逐段处理：
 * 1. 实体区间前的纯文本 → HTML 转义
 * 2. 实体区间 → 按类型包裹标签（内部嵌套递归处理子实体）
 * 3. 最后一段纯文本 → HTML 转义
 *
 * Telegram 的 offset/length 以 UTF-16 码元计，JS 字符串同为 UTF-16，
 * 因此索引可直接对应，无需转换。
 */

const HTML_ESCAPE_MAP: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
}

function escapeHtml(str: string): string {
  return str.replace(/[&<>"']/g, (ch) => HTML_ESCAPE_MAP[ch])
}

/**
 * 按实体类型生成开/闭标签。
 * 返回 [openTag, closeTag]。
 */
function tagsFor(entity: MessageEntity): [string, string] {
  switch (entity.type) {
    case 'bold':
      return ['<b>', '</b>']
    case 'italic':
      return ['<i>', '</i>']
    case 'underline':
      return ['<u>', '</u>']
    case 'strike':
      return ['<s>', '</s>']
    case 'spoiler':
      return ['<span class="spoiler">', '</span>']
    case 'code':
      return ['<code>', '</code>']
    case 'pre':
      return ['<pre>', '</pre>']
    case 'blockquote':
      return ['<blockquote>', '</blockquote>']
    case 'url':
      // 纯 URL 实体，文本本身就是链接
      return [`<a href="${escapeHtml(entity.url || '')}" target="_blank" rel="noopener noreferrer">`, '</a>']
    case 'text_url':
      // 带自定义链接目标的文本
      return [`<a href="${escapeHtml(entity.url || '')}" target="_blank" rel="noopener noreferrer">`, '</a>']
    case 'mention':
      return ['<span class="mention">', '</span>']
    case 'hashtag':
      return ['<span class="hashtag">', '</span>']
    case 'email':
      return [`<a href="mailto:${escapeHtml(entity.url || '')}">`, '</a>']
    case 'phone':
      return [`<a href="tel:${escapeHtml(entity.url || '')}">`, '</a>']
    case 'bot_command':
      return ['<span class="bot-command">', '</span>']
    case 'custom_emoji':
      return ['<span class="custom-emoji">', '</span>']
    default:
      return ['<span>', '</span>']
  }
}

/**
 * 渲染带有实体的消息文本为 HTML。
 *
 * @param text 消息原始文本
 * @param entities 消息实体列表（可为空）
 * @returns 安全的 HTML 字符串
 */
export function renderEntities(text: string, entities?: MessageEntity[] | null): string {
  if (!entities || entities.length === 0) {
    return escapeHtml(text)
  }

  // 按 offset 排序，offset 相同时按 length 降序（外层实体先处理）
  const sorted = [...entities].sort((a, b) => a.offset - b.offset || b.length - a.length)

  let result = ''
  let cursor = 0

  for (const entity of sorted) {
    const { offset, length } = entity

    // 跳过无效或与前一个实体重叠的区间
    if (offset < cursor || length <= 0) continue

    // 实体前的纯文本
    if (offset > cursor) {
      result += escapeHtml(text.slice(cursor, offset))
    }

    // 实体区间
    const entityEnd = Math.min(offset + length, text.length)
    const entityText = text.slice(offset, entityEnd)

    // 对于链接类实体，文本本身可能是 URL，需要转义后作为 href
    if (entity.type === 'url') {
      entity.url = entityText
    }

    const [openTag, closeTag] = tagsFor(entity)
    result += openTag + escapeHtml(entityText) + closeTag

    cursor = entityEnd
  }

  // 最后的纯文本
  if (cursor < text.length) {
    result += escapeHtml(text.slice(cursor))
  }

  return result
}

/**
 * 生成安全的 HTML，同时处理实体和链接。
 * 优先使用实体信息，无实体时回退到简单的 URL 检测。
 */
export function renderMessageText(text: string, entities?: MessageEntity[] | null): string {
  if (entities && entities.length > 0) {
    return renderEntities(text, entities)
  }
  // 无实体时回退到简单的 URL 链接化
  return linkify(escapeHtml(text))
}

function linkify(escaped: string): string {
  return escaped.replace(
    /(https?:\/\/[^\s<]+)/g,
    '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>'
  )
}
