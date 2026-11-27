import DOMPurify, { type Config, type DOMPurify as Purifier } from 'dompurify';

import { isExternalHttpLink, safeImageHref, safeIssueHref, safeLinkHref } from './urls';

/**
 * Strict allow-list for rich text. Raw HTML never reaches this list (the renderer drops it),
 * so the list only admits elements that the Markdown renderer produces.
 */
export const RICH_TEXT_ALLOWED_TAGS: readonly string[] = [
  'a',
  'blockquote',
  'br',
  'button',
  'code',
  'del',
  'em',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'hr',
  'img',
  'input',
  'li',
  'ol',
  'p',
  'pre',
  'span',
  'strong',
  'table',
  'tbody',
  'td',
  'th',
  'thead',
  'tr',
  'ul',
];

/** Attributes the renderer produces. Unknown data-* attributes are dropped. */
export const RICH_TEXT_ALLOWED_ATTRIBUTES: readonly string[] = [
  'align',
  'alt',
  'aria-hidden',
  'aria-label',
  'checked',
  'class',
  'data-issue-key',
  'data-issue-resolved',
  'data-issue-summary',
  'data-rt-image',
  'data-testid',
  'data-user-id',
  'data-user-name',
  'disabled',
  'href',
  'role',
  'src',
  'start',
  'title',
  'type',
];

/** Only relative app paths (single slash), http, https and mailto survive attribute URL checks. */
const ALLOWED_URI = /^(?:https?:|mailto:|\/(?!\/))/i;

const CONFIG: Config & { RETURN_DOM_FRAGMENT: true } = {
  ALLOWED_TAGS: [...RICH_TEXT_ALLOWED_TAGS],
  ALLOWED_ATTR: [...RICH_TEXT_ALLOWED_ATTRIBUTES],
  ALLOWED_URI_REGEXP: ALLOWED_URI,
  // Only href and src carry URLs; the other attributes hold text or identifiers and skip the URL check.
  ADD_URI_SAFE_ATTR: RICH_TEXT_ALLOWED_ATTRIBUTES.filter(
    (name) => name !== 'href' && name !== 'src',
  ),
  ALLOW_DATA_ATTR: false,
  RETURN_DOM_FRAGMENT: true,
};

let purifier: Purifier | undefined;

/** One private DOMPurify instance, so the hooks below do not leak into other sanitizer users. */
function getPurifier(): Purifier {
  if (!purifier) {
    const instance = DOMPurify(window);
    instance.addHook('uponSanitizeAttribute', (node, data) => {
      const el = node as Element;
      const tag = el.tagName.toLowerCase();
      const value = data.attrValue;
      switch (data.attrName) {
        case 'href': {
          if (tag !== 'a') break;
          const safe = el.hasAttribute('data-issue-key')
            ? safeIssueHref(value)
            : safeLinkHref(value);
          if (safe) {
            data.attrValue = safe;
          } else {
            data.keepAttr = false;
          }
          break;
        }
        case 'src':
          if (tag !== 'img' || !safeImageHref(value)) data.keepAttr = false;
          break;
        case 'type':
          // Only checkboxes (task lists) and plain buttons (mentions, images) are allowed.
          if (!(
            (tag === 'input' && value === 'checkbox') ||
            (tag === 'button' && value === 'button')
          )) {
            data.keepAttr = false;
          }
          break;
        case 'target':
        case 'rel':
          // Set after sanitizing, from the final href.
          data.keepAttr = false;
          break;
        default:
          break;
      }
    });
    instance.addHook('afterSanitizeAttributes', (node) => {
      const el = node as Element;
      if (el.tagName.toLowerCase() !== 'a') return;
      const href = el.getAttribute('href') ?? '';
      if (isExternalHttpLink(href)) {
        el.setAttribute('target', '_blank');
        el.setAttribute('rel', 'noopener noreferrer');
      }
    });
    purifier = instance;
  }
  return purifier;
}

/** Sanitizes renderer output and returns it as a detached DOM fragment ready to be inserted. */
export function sanitizeToFragment(html: string): DocumentFragment {
  return getPurifier().sanitize(html, CONFIG) as DocumentFragment;
}
