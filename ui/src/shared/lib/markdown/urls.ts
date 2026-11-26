const EXTERNAL_LINK_RE = /^(https?:\/\/|mailto:)/i
const HTTP_RE = /^https?:\/\//i

/** Returns the href when it is http, https or mailto; otherwise undefined. */
export function safeLinkHref(href: string | null | undefined): string | undefined {
  if (!href) return undefined
  const trimmed = href.trim()
  return EXTERNAL_LINK_RE.test(trimmed) ? trimmed : undefined
}

/** True for links that leave the app and must open in a new tab. */
export function isExternalHttpLink(href: string): boolean {
  return HTTP_RE.test(href)
}

/** Issue hrefs: app-relative paths (not protocol-relative) or http(s). */
export function safeIssueHref(href: string | null | undefined): string | undefined {
  if (!href) return undefined
  const trimmed = href.trim()
  if (trimmed.startsWith('/') && !trimmed.startsWith('//')) return trimmed
  return HTTP_RE.test(trimmed) ? trimmed : undefined
}
