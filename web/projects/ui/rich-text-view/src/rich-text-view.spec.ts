import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import type { IssueResolver } from '@corelog/core';

import { RichTextView } from './rich-text-view';

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
  ]) {
    expect(container.querySelectorAll(tag), tag).toHaveLength(0);
  }
  for (const el of Array.from(container.querySelectorAll('*'))) {
    for (const attr of Array.from(el.attributes)) {
      expect(attr.name.startsWith('on'), `${el.tagName} ${attr.name}`).toBe(false);
    }
  }
  for (const el of Array.from(container.querySelectorAll('[href],[src]'))) {
    const url = (el.getAttribute('href') ?? el.getAttribute('src') ?? '').trim().toLowerCase();
    expect(/^(javascript|data|vbscript):/.test(url), url).toBe(false);
  }
}

@Component({
  imports: [RichTextView],
  template: `
    <app-rich-text-view
      [value]="value()"
      [resolveIssue]="resolveIssue()"
      [mentionsClickable]="clickable()"
      (mentionClick)="clicked.push($event)"
    />
  `,
})
class HostComponent {
  readonly value = signal('');
  readonly resolveIssue = signal<IssueResolver | undefined>(undefined);
  readonly clickable = signal(false);
  readonly clicked: string[] = [];
}

async function render(
  value: string,
  options: { resolveIssue?: IssueResolver; clickable?: boolean } = {},
): Promise<{ host: HostComponent; el: HTMLElement; fixture: ComponentFixture<HostComponent> }> {
  TestBed.configureTestingModule({ imports: [HostComponent], providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(HostComponent);
  fixture.componentInstance.value.set(value);
  if (options.resolveIssue) fixture.componentInstance.resolveIssue.set(options.resolveIssue);
  if (options.clickable) fixture.componentInstance.clickable.set(true);
  fixture.detectChanges();
  await fixture.whenStable();
  return { host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement, fixture };
}

function query(el: HTMLElement, selector: string): Element | null {
  return el.querySelector(selector);
}

describe('RichTextView XSS corpus', () => {
  it.each(XSS_CORPUS)('renders $name inert', async ({ markdown }) => {
    const { el } = await render(markdown);
    expectInert(el);
  });

  it('drops unsafe issue hrefs and keeps the key as text', async () => {
    const { el } = await render('Mira CL-5', {
      resolveIssue: () => ({ href: 'javascript:alert(1)', summary: 's', resolved: false }),
    });
    expectInert(el);
    expect(query(el, 'a[data-issue-key]')).toBeNull();
    expect(el.textContent).toContain('Mira CL-5');
  });
});

describe('RichTextView links and images', () => {
  it('opens http links in a new tab with safe rel', async () => {
    const { el } = await render('[Docs](https://example.com/docs)');
    const link = el.querySelector('a');
    expect(link?.getAttribute('href')).toBe('https://example.com/docs');
    expect(link?.getAttribute('target')).toBe('_blank');
    expect(link?.getAttribute('rel')).toBe('noopener noreferrer');
  });

  it('allows mailto without opening a new tab', async () => {
    const { el } = await render('[Escribir](mailto:ana@example.com)');
    const link = el.querySelector('a');
    expect(link?.getAttribute('href')).toBe('mailto:ana@example.com');
    expect(link?.hasAttribute('target')).toBe(false);
  });

  it('renders http images as a button that opens a lightbox', async () => {
    const { el, fixture } = await render('![Diagrama](https://cdn.example.com/d.png)');
    screen(el, 'Ver imagen: Diagrama').click();
    fixture.detectChanges();
    const dialog = el.querySelector('[role="dialog"]');
    expect(dialog).not.toBeNull();
  });
});

describe('RichTextView GFM', () => {
  it('renders tables, task lists and code blocks', async () => {
    const { el } = await render(
      '| a | b |\n| - | - |\n| 1 | 2 |\n\n- [x] hecho\n- [ ] pendiente\n\n```ts\nconst a = 1\n```',
    );
    expect(el.querySelector('table')).not.toBeNull();
    expect(el.querySelectorAll('input[type="checkbox"]')).toHaveLength(2);
    expect(query(el, 'pre code')?.textContent).toContain('const a = 1');
  });
});

describe('RichTextView mentions', () => {
  it('renders mentions as chips without a link', async () => {
    const { el } = await render(`Hola @[Ana Pérez](user:${UID})`);
    expect(query(el, '[data-testid="rich-text-mention"]')?.textContent).toContain('Ana Pérez');
    expect(el.querySelector('a')).toBeNull();
    expect(screen(el, 'Ana Pérez', 'img')).not.toBeNull();
  });

  it('emits mentionClick with the user id when a chip is clicked', async () => {
    const { host, el } = await render(`@[Ana](user:${UID})`, { clickable: true });
    const button = el.querySelector('button[data-user-id]') as HTMLButtonElement;
    button.click();
    expect(host.clicked).toEqual([UID]);
  });
});

describe('RichTextView issue keys', () => {
  it('autolinks resolved keys and leaves unknown ones as text', async () => {
    const resolveIssue = vi.fn((key: string) =>
      key === 'CL-12'
        ? { href: '/incidencias/CL-12', summary: 'Error al guardar', resolved: true }
        : undefined,
    );
    const { el } = await render('Revisa CL-12 y CL-99', { resolveIssue });
    const link = el.querySelector('a[data-issue-key="CL-12"]') as HTMLAnchorElement;
    expect(link.getAttribute('href')).toBe('/incidencias/CL-12');
    expect(link.className).toMatch(/resolved/);
    expect(el.querySelector('a[data-issue-key="CL-99"]')).toBeNull();
    expect(el.textContent).toContain('CL-99');
    expect(resolveIssue).toHaveBeenCalledWith('CL-99');
  });

  it('does not autolink keys inside code', async () => {
    const { el } = await render('`CL-12`', {
      resolveIssue: () => ({ href: '/x', summary: '', resolved: false }),
    });
    expect(el.querySelector('a')).toBeNull();
    expect(query(el, 'code')?.textContent).toBe('CL-12');
  });
});

describe('RichTextView lightbox', () => {
  it('closes with Escape from the dialog', async () => {
    const { el, fixture } = await render('![Foto](https://cdn.example.com/f.png)');
    screen(el, 'Ver imagen: Foto').click();
    fixture.detectChanges();
    const dialog = el.querySelector('[role="dialog"]') as HTMLElement;
    expect(dialog).not.toBeNull();
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();
    expect(el.querySelector('[role="dialog"]')).toBeNull();
  });
});

/** Finds an element by aria-label, or by a text fragment for the given role. */
function screen(el: HTMLElement, name: string, role?: string): HTMLElement {
  if (role === 'img') {
    return el.querySelector(`[role="img"][aria-label="${name}"]`) as HTMLElement;
  }
  return el.querySelector(`[aria-label="${name}"]`) as HTMLElement;
}
