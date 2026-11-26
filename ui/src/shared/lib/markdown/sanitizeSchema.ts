import { defaultSchema } from 'rehype-sanitize'
import type { Options } from 'rehype-sanitize'

/**
 * Strict sanitizer schema for rich text. Raw HTML is never parsed (no rehype-raw).
 * Links allow http, https and mailto only; images allow http and https only.
 * `span` may carry the data attributes produced by remarkCorelog (mentions and issue keys);
 * their values are rendered as text or validated by the view, never as markup.
 */
export const richTextSchema: Options = {
  ...defaultSchema,
  protocols: {
    ...defaultSchema.protocols,
    href: ['http', 'https', 'mailto'],
    src: ['http', 'https'],
  },
  attributes: {
    ...defaultSchema.attributes,
    span: [
      'dataUserId',
      'dataUserName',
      'dataIssueKey',
      'dataIssueHref',
      'dataIssueSummary',
      'dataIssueResolved',
    ],
  },
}
