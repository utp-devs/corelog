import { isUserId } from './mentions'
import { splitIssueKeys } from './issueKeys'

/**
 * Resolution of an issue key for rendering. Returning undefined keeps the key as plain text.
 */
export type IssueResolution = {
  href: string
  summary: string
  resolved: boolean
}

export type IssueResolver = (key: string) => IssueResolution | undefined

/** Minimal structural view of an mdast node; only the fields this plugin touches. */
type MdNode = {
  type: string
  value?: string
  url?: string
  children?: MdNode[]
  data?: Record<string, unknown>
}

const SKIP_TYPES = new Set(['link', 'inlineCode', 'code', 'html'])

function mentionNode(userId: string, name: string): MdNode {
  return {
    type: 'corelogMention',
    data: { hName: 'span', hProperties: { dataUserId: userId, dataUserName: name } },
    children: [{ type: 'text', value: name }],
  }
}

function issueNode(key: string, resolution: IssueResolution): MdNode {
  return {
    type: 'corelogIssue',
    data: {
      hName: 'span',
      hProperties: {
        dataIssueKey: key,
        dataIssueHref: resolution.href,
        dataIssueSummary: resolution.summary,
        dataIssueResolved: resolution.resolved ? 'true' : 'false',
      },
    },
    children: [{ type: 'text', value: key }],
  }
}

/** Turns a text node into text and issue nodes; unresolved keys stay as text. */
function splitTextNode(value: string, resolve: IssueResolver | undefined): MdNode[] {
  if (!resolve) {
    return [{ type: 'text', value }]
  }
  const nodes: MdNode[] = []
  for (const segment of splitIssueKeys(value)) {
    if (segment.type === 'text') {
      nodes.push({ type: 'text', value: segment.value })
      continue
    }
    const resolution = resolve(segment.key)
    nodes.push(resolution ? issueNode(segment.key, resolution) : { type: 'text', value: segment.key })
  }
  return nodes
}

function appendText(list: MdNode[], node: MdNode) {
  const prev = list[list.length - 1]
  if (node.type === 'text' && prev?.type === 'text') {
    prev.value = `${prev.value ?? ''}${node.value ?? ''}`
    return
  }
  list.push(node)
}

function transformChildren(parent: MdNode, resolve: IssueResolver | undefined) {
  if (!parent.children) return
  const next: MdNode[] = []
  for (const child of parent.children) {
    if (child.type === 'link' && child.url?.startsWith('user:')) {
      const userId = child.url.slice('user:'.length)
      // Remark parses "@[Name](user:UUID)" as text "@" followed by a link: drop the "@".
      const prev = next[next.length - 1]
      if (prev?.type === 'text' && prev.value?.endsWith('@')) {
        prev.value = prev.value.slice(0, -1)
        if (prev.value === '') next.pop()
      }
      if (isUserId(userId)) {
        const name = (child.children ?? []).map((c) => c.value ?? '').join('') || 'Usuario'
        next.push(mentionNode(userId, name))
      } else {
        // Invalid mention target: keep the label as plain text.
        for (const grand of child.children ?? []) appendText(next, grand)
      }
      continue
    }
    if (child.type === 'text' && typeof child.value === 'string') {
      for (const node of splitTextNode(child.value, resolve)) appendText(next, node)
      continue
    }
    if (!SKIP_TYPES.has(child.type)) {
      transformChildren(child, resolve)
    }
    next.push(child)
  }
  parent.children = next
}

/**
 * Remark plugin: converts mention links (user:UUID) into span nodes with user data,
 * and issue keys into span nodes when resolveIssue returns a resolution.
 * Everything else is left to the sanitizer.
 */
export function remarkCorelog(resolveIssue?: IssueResolver) {
  return () => (tree: unknown) => {
    transformChildren(tree as MdNode, resolveIssue)
  }
}
