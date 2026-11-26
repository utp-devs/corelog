/** Mention token syntax stored in Markdown: @[Display Name](user:UUID). */

const UUID_SOURCE = '[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
const UUID_RE = new RegExp(`^${UUID_SOURCE}$`)
const MENTION_RE = new RegExp(`@\\[([^\\[\\]\\r\\n]+)\\]\\(user:(${UUID_SOURCE})\\)`, 'g')

export type MentionUser = {
  id: string
  name: string
}

export type TextSegment = { type: 'text'; value: string }
export type MentionSegment = { type: 'mention'; userId: string; name: string }
export type MentionParseSegment = TextSegment | MentionSegment

export function isUserId(value: string): boolean {
  return UUID_RE.test(value)
}

/** Builds the stored token for a user. Characters that would break the syntax are replaced by spaces. */
export function mentionToken(user: MentionUser): string {
  if (!isUserId(user.id)) {
    throw new RangeError(`mentionToken: invalid user id "${user.id}"`)
  }
  const name = user.name.replace(/[[\]()\r\n]/g, ' ').replace(/\s+/g, ' ').trim() || 'Usuario'
  return `@[${name}](user:${user.id})`
}

/** Splits text into plain segments and mention segments, in order. Invalid tokens stay as text. */
export function parseMentions(text: string): MentionParseSegment[] {
  const segments: MentionParseSegment[] = []
  let last = 0
  for (const match of text.matchAll(MENTION_RE)) {
    const index = match.index ?? 0
    if (index > last) {
      segments.push({ type: 'text', value: text.slice(last, index) })
    }
    segments.push({ type: 'mention', name: match[1], userId: match[2] })
    last = index + match[0].length
  }
  if (last < text.length) {
    segments.push({ type: 'text', value: text.slice(last) })
  }
  return segments
}
