import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { RichTextView } from './RichTextView'

const UID = '3f2a9c1e-7b4d-4e8a-9c2f-1a2b3c4d5e6f'

const XSS_CORPUS: { name: string; markdown: string }[] = [
  { name: 'script tag', markdown: '<script>alert(1)</script>' },
  { name: 'javascript link', markdown: '[x](javascript:alert(1))' },
  { name: 'javascript link mixed case', markdown: '[x](JaVaScRiPt:alert(1))' },
  { name: 'data link', markdown: '[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)' },
  { name: 'vbscript link', markdown: '[x](vbscript:msgbox(1))' },
  { name: 'img onerror raw', markdown: '<img src=x onerror=alert(1)>' },
  { name: 'img onerror quoted', markdown: '<img src="x" onerror="alert(1)">' },
  { name: 'anchor with onclick', markdown: '<a href="javascript:alert(1)" onclick="alert(1)">x</a>' },
  { name: 'iframe', markdown: '<iframe src="javascript:alert(1)"></iframe>' },
  { name: 'svg with script', markdown: '<svg onload=alert(1)><script>alert(1)</script></svg>' },
  { name: 'markdown image javascript', markdown: '![x](javascript:alert(1))' },
  { name: 'markdown image data svg', markdown: '![x](data:image/svg+xml;base64,PHN2Zz4=)' },
  { name: 'nested html in table', markdown: '| a | b |\n| - | - |\n| <img src=x onerror=alert(1)> | <script>x</script> |' },
  { name: 'div event handler', markdown: '<div onmouseover="alert(1)">hi</div>' },
  { name: 'style block', markdown: '<style>body{display:none}</style>' },
  { name: 'object element', markdown: '<object data="javascript:alert(1)"></object>' },
  { name: 'mention name with html', markdown: `@[<img src=x onerror=alert(1)>](user:${UID})` },
  { name: 'mention with invalid target', markdown: '@[<b>x</b>](user:javascript:alert(1))' },
]

/** Asserts that the rendered DOM holds no executable or unsafe constructs. */
function expectInert(container: HTMLElement) {
  for (const tag of ['script', 'iframe', 'style', 'object', 'embed', 'svg', 'noscript', 'template']) {
    expect(container.querySelectorAll(tag), tag).toHaveLength(0)
  }
  for (const el of Array.from(container.querySelectorAll('*'))) {
    for (const attr of Array.from(el.attributes)) {
      expect(attr.name.startsWith('on'), `${el.tagName} ${attr.name}`).toBe(false)
    }
  }
  for (const el of Array.from(container.querySelectorAll('[href],[src]'))) {
    const url = (el.getAttribute('href') ?? el.getAttribute('src') ?? '').trim().toLowerCase()
    expect(/^(javascript|data|vbscript):/.test(url), url).toBe(false)
  }
}

describe('RichTextView XSS corpus', () => {
  it.each(XSS_CORPUS)('renders $name inert', ({ markdown }) => {
    const { container, unmount } = render(<RichTextView value={markdown} />)
    expectInert(container)
    unmount()
  })

  it('drops unsafe issue hrefs and keeps the key as text', () => {
    const { container } = render(
      <RichTextView value="Mira CL-5" resolveIssue={() => ({ href: 'javascript:alert(1)', summary: 's', resolved: false })} />,
    )
    expectInert(container)
    expect(screen.queryByRole('link', { name: 'CL-5' })).not.toBeInTheDocument()
    expect(container).toHaveTextContent('Mira CL-5')
  })
})

describe('RichTextView links and images', () => {
  it('opens http links in a new tab with safe rel', () => {
    render(<RichTextView value="[Docs](https://example.com/docs)" />)
    const link = screen.getByRole('link', { name: 'Docs' })
    expect(link).toHaveAttribute('href', 'https://example.com/docs')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('allows mailto without opening a new tab', () => {
    render(<RichTextView value="[Escribir](mailto:ana@example.com)" />)
    const link = screen.getByRole('link', { name: 'Escribir' })
    expect(link).toHaveAttribute('href', 'mailto:ana@example.com')
    expect(link).not.toHaveAttribute('target')
  })

  it('renders http images as a button that opens a lightbox', async () => {
    render(<RichTextView value="![Diagrama](https://cdn.example.com/d.png)" />)
    fireEvent.click(screen.getByRole('button', { name: 'Ver imagen: Diagrama' }))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })
})

describe('RichTextView GFM', () => {
  it('renders tables, task lists and code blocks', () => {
    const { container } = render(
      <RichTextView value={'| a | b |\n| - | - |\n| 1 | 2 |\n\n- [x] hecho\n- [ ] pendiente\n\n```ts\nconst a = 1\n```'} />,
    )
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(container.querySelectorAll('input[type="checkbox"]')).toHaveLength(2)
    expect(container.querySelector('pre code')).toHaveTextContent('const a = 1')
  })
})

describe('RichTextView mentions', () => {
  it('renders mentions as chips without a link', () => {
    render(<RichTextView value={`Hola @[Ana Pérez](user:${UID})`} />)
    expect(screen.getByTestId('rich-text-mention')).toHaveTextContent('Ana Pérez')
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Ana Pérez' })).toBeInTheDocument()
  })

  it('calls onMentionClick with the user id when a chip is clicked', () => {
    const onMentionClick = vi.fn()
    render(<RichTextView value={`@[Ana](user:${UID})`} onMentionClick={onMentionClick} />)
    fireEvent.click(screen.getByRole('button', { name: /Ana/ }))
    expect(onMentionClick).toHaveBeenCalledWith(UID)
  })
})

describe('RichTextView issue keys', () => {
  it('autolinks resolved keys and leaves unknown ones as text', () => {
    const resolveIssue = vi.fn((key: string) =>
      key === 'CL-12' ? { href: '/incidencias/CL-12', summary: 'Error al guardar', resolved: true } : undefined,
    )
    render(<RichTextView value="Revisa CL-12 y CL-99" resolveIssue={resolveIssue} />)
    const link = screen.getByRole('link', { name: 'CL-12' })
    expect(link).toHaveAttribute('href', '/incidencias/CL-12')
    expect(link.className).toMatch(/resolved/)
    expect(screen.queryByRole('link', { name: 'CL-99' })).not.toBeInTheDocument()
    expect(screen.getByText(/CL-99/)).toBeInTheDocument()
    expect(resolveIssue).toHaveBeenCalledWith('CL-99')
  })

  it('does not autolink keys inside code', () => {
    render(<RichTextView value="`CL-12`" resolveIssue={() => ({ href: '/x', summary: '', resolved: false })} />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByText('CL-12').tagName).toBe('CODE')
  })
})
