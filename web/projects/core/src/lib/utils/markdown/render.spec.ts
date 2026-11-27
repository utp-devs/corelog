import { renderMarkdown, renderMarkdownFragment } from './index';

const UID = '3f2a9c1e-7b4d-4e8a-9c2f-1a2b3c4d5e6f';

const XSS_CORPUS: { name: string; markdown: string }[] = [
  { name: 'script tag', markdown: '<script>alert(1)</script>' },
  { name: 'javascript link', markdown: '[x](javascript:alert(1))' },
  { name: 'javascript link mixed case', markdown: '[x](JaVaScRiPt:alert(1))' },
  {
    name: 'data link',
    markdown: '[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)',
  },
  { name: 'vbscript link', markdown: '[x](vbscript:msgbox(1))' },
  { name: 'img onerror raw', markdown: '<img src=x onerror=alert(1)>' },
  { name: 'img onerror quoted', markdown: '<img src="x" onerror="alert(1)">' },
  {
    name: 'anchor with onclick',
    markdown: '<a href="javascript:alert(1)" onclick="alert(1)">x</a>',
  },
  { name: 'iframe', markdown: '<iframe src="javascript:alert(1)"></iframe>' },
  { name: 'svg with script', markdown: '<svg onload=alert(1)><script>alert(1)</script></svg>' },
  { name: 'markdown image javascript', markdown: '![x](javascript:alert(1))' },
  { name: 'markdown image data svg', markdown: '![x](data:image/svg+xml;base64,PHN2Zz4=)' },
  {
    name: 'nested html in table',
    markdown: '| a | b |\n| - | - |\n| <img src=x onerror=alert(1)> | <script>x</script> |',
  },
  { name: 'div event handler', markdown: '<div onmouseover="alert(1)">hi</div>' },
  { name: 'style block', markdown: '<style>body{display:none}</style>' },
  { name: 'object element', markdown: '<object data="javascript:alert(1)"></object>' },
  { name: 'mention name with html', markdown: `@[<img src=x onerror=alert(1)>](user:${UID})` },
  { name: 'mention with invalid target', markdown: '@[<b>x</b>](user:javascript:alert(1))' },
  { name: 'obfuscated javascript link with entities', markdown: '[x](&#106;avascript:alert(1))' },
  { name: 'javascript link with whitespace', markdown: '[x](  javascript:alert(1))' },
  { name: 'autolink-looking javascript', markdown: '<javascript:alert(1)>' },
  { name: 'mailto with script in label', markdown: '[<script>x</script>](mailto:a@example.com)' },
  { name: 'nested emphasis with payload', markdown: '**_<img src=x onerror=alert(1)>_**' },
  { name: 'reference link javascript', markdown: '[x][r]\n\n[r]: javascript:alert(1)' },
  {
    name: 'form and button',
    markdown:
      '<form action="javascript:alert(1)"><button formaction="javascript:alert(1)">x</button></form>',
  },
  {
    name: 'base and meta refresh',
    markdown:
      '<base href="javascript:alert(1)"><meta http-equiv="refresh" content="0;url=javascript:alert(1)">',
  },
];

/** Asserts that the rendered DOM holds no executable or unsafe constructs. */
function expectInert(container: HTMLElement) {
  for (const tag of [
    'script',
    'iframe',
    'style',
    'object',
    'embed',
    'svg',
    'noscript',
    'template',
    'form',
    'base',
    'meta',
  ]) {
    expect(container.querySelectorAll(tag), tag).toHaveLength(0);
  }
  for (const el of Array.from(container.querySelectorAll('*'))) {
    for (const attr of Array.from(el.attributes)) {
      expect(attr.name.startsWith('on'), `${el.tagName} ${attr.name}`).toBe(false);
    }
  }
  for (const el of Array.from(container.querySelectorAll('[href],[src],[action],[formaction]'))) {
    const url = (el.getAttribute('href') ?? el.getAttribute('src') ?? '')
      .replace(/\s/g, '')
      .toLowerCase();
    expect(/^(javascript|data|vbscript):/.test(url), url).toBe(false);
  }
}

function inRoot(html: string): HTMLElement {
  const root = document.createElement('div');
  root.innerHTML = html;
  return root;
}

describe('markdown XSS corpus', () => {
  it.each(XSS_CORPUS)('renders $name inert', ({ markdown }) => {
    const fragment = renderMarkdownFragment(markdown);
    const container = document.createElement('div');
    container.append(fragment);
    expectInert(container);
  });

  it('drops unsafe issue hrefs and keeps the key as text', () => {
    const html = renderMarkdown('Mira CL-5', {
      resolveIssue: () => ({ href: 'javascript:alert(1)', summary: 's', resolved: false }),
    });
    const container = inRoot(html);
    expectInert(container);
    expect(container.querySelector('a')).toBeNull();
    expect(container.textContent).toContain('Mira CL-5');
  });
});

describe('markdown renderer behavior', () => {
  it('serializes sanitized output with safe external links', () => {
    const container = inRoot(renderMarkdown('[Docs](https://example.com/docs)'));
    const link = container.querySelector('a');
    expect(link?.getAttribute('href')).toBe('https://example.com/docs');
    expect(link?.getAttribute('target')).toBe('_blank');
    expect(link?.getAttribute('rel')).toBe('noopener noreferrer');
  });

  it('keeps mailto links without a new tab', () => {
    const link = inRoot(renderMarkdown('[Escribir](mailto:ana@example.com)')).querySelector('a');
    expect(link?.getAttribute('href')).toBe('mailto:ana@example.com');
    expect(link?.hasAttribute('target')).toBe(false);
  });

  it('keeps raw HTML out of the output', () => {
    expect(renderMarkdown('texto <b>fuerte</b>')).not.toContain('<b>');
  });

  it('does not link issue keys inside code', () => {
    const container = inRoot(
      renderMarkdown('`CL-12`', {
        resolveIssue: () => ({ href: '/x', summary: '', resolved: false }),
      }),
    );
    expect(container.querySelector('a')).toBeNull();
    expect(container.querySelector('code')?.textContent).toBe('CL-12');
  });

  it('keeps the mention name as escaped text inside the chip', () => {
    const container = inRoot(renderMarkdown(`@[<i>Ana</i>](user:${UID})`));
    const mention = container.querySelector('[data-user-id]');
    expect(mention?.getAttribute('data-user-id')).toBe(UID);
    expect(mention?.getAttribute('data-user-name')).toBe('<i>Ana</i>');
    expect(container.querySelector('i')).toBeNull();
  });

  it('renders the mention as a button when interactive', () => {
    const container = inRoot(renderMarkdown(`@[Ana](user:${UID})`, { interactiveMentions: true }));
    expect(container.querySelector('button[data-user-id]')).not.toBeNull();
  });
});
