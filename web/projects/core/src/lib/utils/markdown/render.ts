import { Marked } from 'marked';

import { corelogMarkdownExtension, type CorelogMarkdownOptions } from './corelog-extension';
import { sanitizeToFragment } from './sanitize';

/**
 * Renders CoreLog Markdown (GFM plus mentions and issue keys) into a sanitized DOM fragment.
 * Insert the fragment with a DOM API; the output is safe to insert as it contains no scripts,
 * event handler attributes, raw HTML or unsafe URLs.
 */
export function renderMarkdownFragment(
  source: string,
  options: CorelogMarkdownOptions = {},
): DocumentFragment {
  const marked = new Marked(
    { gfm: true, breaks: false, async: false },
    corelogMarkdownExtension(options),
  );
  const html = marked.parse(source, { async: false });
  return sanitizeToFragment(html);
}

/** Sanitized HTML string for the Markdown source. Same output as renderMarkdownFragment, serialized. */
export function renderMarkdown(source: string, options: CorelogMarkdownOptions = {}): string {
  const container = document.createElement('div');
  container.append(renderMarkdownFragment(source, options));
  return container.innerHTML;
}
