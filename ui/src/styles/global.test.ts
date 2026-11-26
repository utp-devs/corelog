import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// jsdom does not apply stylesheets to layout, so these tests pin the mobile hit-area contract
// (minimum tap target size on phones) by reading the source CSS.
const here = dirname(fileURLToPath(import.meta.url))
const css = readFileSync(resolve(here, 'global.css'), 'utf8')

function mobileBlock(): string {
  const start = css.indexOf('@media (max-width: 767.98px)')
  expect(start).toBeGreaterThan(-1)
  return css.slice(start, css.indexOf('}\n\n', css.indexOf('.ant-btn-sm.ant-btn-icon-only')) + 1)
}

describe('global.css hit areas', () => {
  it('keeps icon-only and small antd buttons at least 32 px tall on mobile', () => {
    const block = mobileBlock()
    expect(block).toContain('.ant-btn-sm')
    expect(block).toContain('min-height: var(--cl-hit-min-mobile)')
    expect(block).toContain('.ant-btn-sm.ant-btn-icon-only')
    expect(block).toContain('min-width: var(--cl-hit-min-mobile)')
  })

  it('sizes the toast close control to 24 px on desktop and 32 px on mobile', () => {
    expect(css).toMatch(/\.ant-notification-notice-close \{[^}]*min-width: var\(--cl-hit-min\)/)
    expect(mobileBlock()).toContain('.ant-notification-notice-close')
  })

  it('defines the desktop and mobile minimums as tokens', () => {
    expect(css).toContain('--cl-hit-min: 24px')
    expect(css).toContain('--cl-hit-min-mobile: 32px')
  })
})

const shared = resolve(here, '../shared/ui')
const readShared = (path: string) => readFileSync(resolve(shared, path), 'utf8')

describe('component CSS contracts', () => {
  it('grows color swatches and inline-edit triggers to 32 px below 768 px only', () => {
    for (const path of ['ColorPicker/ColorPicker.module.css', 'InlineEdit/InlineEdit.module.css']) {
      const source = readShared(path)
      const media = source.indexOf('@media (max-width: 767.98px)')
      expect(media, path).toBeGreaterThan(-1)
      expect(source.slice(media), path).toMatch(/32px/)
    }
  })

  it('lets lozenges and tone tags wrap instead of clipping', () => {
    expect(readShared('Lozenge/Lozenge.module.css')).toMatch(/white-space: normal/)
    expect(readShared('Lozenge/Lozenge.module.css')).not.toMatch(/overflow: hidden/)
    expect(readShared('ToneTag.module.css')).toMatch(/white-space: normal/)
  })

  it('keeps toolbar buttons at 32 px and lets their labels wrap', () => {
    const source = readShared('Toolbar/Toolbar.module.css')
    expect(source).toMatch(/min-height: 32px/)
    expect(source).toMatch(/white-space: normal/)
  })
})
