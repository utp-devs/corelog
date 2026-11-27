import type {
  MarkedExtension,
  RendererObject,
  RendererExtension,
  TokenizerExtension,
  Tokens,
} from 'marked';

import { escapeHtml } from './html';
import { splitIssueKeys } from './issue-keys';
import { isUserId } from './mention-token';
import { safeImageHref, safeIssueHref, safeLinkHref } from './urls';

/** Resolution of an issue key for rendering. Returning undefined keeps the key as plain text. */
export interface IssueResolution {
  readonly href: string;
  readonly summary: string;
  readonly resolved: boolean;
}

export type IssueResolver = (key: string) => IssueResolution | undefined;

export interface CorelogMarkdownOptions {
  /** Resolves issue keys to links. Without it, keys stay plain text. */
  readonly resolveIssue?: IssueResolver;
  /** Renders mentions as buttons (clickable) instead of static chips. */
  readonly interactiveMentions?: boolean;
}

/** Mention syntax as typed in the editor: @[Name](user:UUID). Anything else after "@[" is ordinary text. */
const MENTION_START = '@[';
const MENTION_RE = /^@\[([^[\]\r\n]+)\]\(user:([^\r\n]*?)\)/;

function initialsOf(name: string): string {
  const initials = name
    .trim()
    .split(/\s+/)
    .filter((part) => part !== '')
    .slice(0, 2)
    .map((part) => Array.from(part)[0] ?? '')
    .join('')
    .toUpperCase();
  return escapeHtml(initials || '?');
}

function mentionHtml(userId: string, name: string, interactive: boolean): string {
  const shown = name.trim() ? name : 'Usuario';
  const data = `data-user-id="${escapeHtml(userId)}" data-user-name="${escapeHtml(shown)}"`;
  if (interactive) {
    return (
      `<button type="button" class="cl-rt-mention cl-rt-mention--button" ${data}>` +
      `<span class="cl-rt-avatar" aria-hidden="true">${initialsOf(shown)}</span>` +
      `<span class="cl-rt-mention-name">${escapeHtml(shown)}</span></button>`
    );
  }
  return (
    `<span class="cl-rt-mention" data-testid="rich-text-mention" ${data}>` +
    `<span class="cl-rt-avatar" role="img" aria-label="${escapeHtml(shown)}">${initialsOf(shown)}</span>` +
    `<span class="cl-rt-mention-name">${escapeHtml(shown)}</span></span>`
  );
}

function issueHtml(key: string, href: string, resolution: IssueResolution): string {
  const modifier = resolution.resolved ? ' cl-rt-issue--resolved' : '';
  const summary = escapeHtml(resolution.summary);
  return (
    `<a class="cl-rt-issue${modifier}" href="${escapeHtml(href)}" data-issue-key="${escapeHtml(key)}"` +
    ` data-issue-resolved="${resolution.resolved ? 'true' : 'false'}" data-issue-summary="${summary}"` +
    ` title="${summary}">${escapeHtml(key)}</a>`
  );
}

/**
 * Marked extension for CoreLog Markdown:
 * - "@[Name](user:UUID)" becomes a mention (chip or button); an invalid target keeps the label as text.
 * - Issue keys (CL-12) become issue links when resolveIssue returns a resolution. Keys inside links
 *   and code are never linked.
 * - Raw HTML is dropped, links and images must use safe URLs, and every other construct is
 *   rendered by the default renderer. The sanitizer runs on the output as a second layer.
 */
export function corelogMarkdownExtension(options: CorelogMarkdownOptions = {}): MarkedExtension {
  const interactive = options.interactiveMentions === true;
  const resolveIssue = options.resolveIssue;
  let linkDepth = 0;

  const mention: TokenizerExtension & RendererExtension = {
    name: 'corelogMention',
    level: 'inline',
    start(src: string) {
      const index = src.indexOf(MENTION_START);
      return index === -1 ? undefined : index;
    },
    tokenizer(src: string): Tokens.Generic | undefined {
      const match = MENTION_RE.exec(src);
      if (!match) return undefined;
      const [raw, label, target] = match;
      if (isUserId(target)) {
        return { type: 'corelogMention', raw, userId: target, name: label };
      }
      return { type: 'text', raw, text: label };
    },
    renderer(token: Tokens.Generic): string {
      return mentionHtml(String(token['userId']), String(token['name']), interactive);
    },
  };

  const renderer: RendererObject = {
    // Raw HTML is never rendered.
    html() {
      return '';
    },
    image(token: Tokens.Image) {
      const src = safeImageHref(token.href);
      if (!src) return '';
      const alt = token.text;
      const label = alt ? `Ver imagen: ${alt}` : 'Ver imagen';
      return (
        `<button type="button" class="cl-rt-image-button" data-rt-image aria-label="${escapeHtml(label)}">` +
        `<img class="cl-rt-image" src="${escapeHtml(src)}" alt="${escapeHtml(alt)}"></button>`
      );
    },
    link(token: Tokens.Link) {
      linkDepth += 1;
      let inner: string;
      try {
        inner = this.parser.parseInline(token.tokens);
      } finally {
        linkDepth -= 1;
      }
      const href = safeLinkHref(token.href);
      if (!href) {
        return `<span class="cl-rt-unsafe-link">${inner}</span>`;
      }
      return `<a class="cl-rt-link" href="${escapeHtml(href)}">${inner}</a>`;
    },
    text(token: Tokens.Text | Tokens.Escape) {
      if ('tokens' in token && token.tokens) {
        return this.parser.parseInline(token.tokens);
      }
      const plain = 'escaped' in token && token.escaped ? token.text : escapeHtml(token.text);
      if (!resolveIssue || linkDepth > 0 || !token.text) {
        return plain;
      }
      let out = '';
      for (const segment of splitIssueKeys(token.text)) {
        if (segment.type === 'text') {
          out += escapeHtml(segment.value);
          continue;
        }
        const resolution = resolveIssue(segment.key);
        const href = resolution ? safeIssueHref(resolution.href) : undefined;
        out +=
          resolution && href ? issueHtml(segment.key, href, resolution) : escapeHtml(segment.key);
      }
      return out;
    },
  };

  return { extensions: [mention], renderer };
}
