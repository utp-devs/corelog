/// <reference types="node" />
// Design token guard.
// Parses tokens.css, compares it with the TS mirror in shared/theme/tokens.ts
// and checks WCAG 2.1 contrast of the declared pairs in both themes.
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { themeTokens } from '@/shared/theme/tokens'

// Vitest runs from ui/ (npm test), so the path is relative to the cwd.
// import.meta.url is not a file URL under the jsdom environment.
const css = readFileSync(resolve(process.cwd(), 'src/styles/tokens.css'), 'utf8')

function parseDeclarations(body: string): Map<string, string> {
  const out = new Map<string, string>()
  const clean = body.replace(/\/\*[\s\S]*?\*\//g, '')
  for (const decl of clean.split(';')) {
    const idx = decl.indexOf(':')
    if (idx === -1) continue
    const name = decl.slice(0, idx).trim()
    if (!name.startsWith('--')) continue
    out.set(name, decl.slice(idx + 1).trim())
  }
  return out
}

function topLevelBlock(selector: string): Map<string, string> {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  // Anchored at line start so the indented @media override is not matched.
  const match = css.match(new RegExp(`^${escaped}\\s*\\{([^}]*)\\}`, 'm'))
  if (!match) throw new Error(`block not found: ${selector}`)
  return parseDeclarations(match[1])
}

const root = topLevelBlock(':root')
const dark = topLevelBlock(":root[data-theme='dark']")
const mobileGutter = css.match(
  /@media\s*\(max-width:\s*767\.98px\)\s*\{\s*:root\s*\{([^}]*)\}/,
)

const existingThemeTokens = [
  '--cl-bg-page', '--cl-bg-surface', '--cl-bg-sunken', '--cl-border', '--cl-border-strong',
  '--cl-text', '--cl-text-subtle', '--cl-text-disabled',
  '--cl-brand', '--cl-brand-hover', '--cl-brand-pressed', '--cl-brand-subtle',
  '--cl-success', '--cl-success-bg', '--cl-success-text',
  '--cl-warning', '--cl-warning-bg', '--cl-warning-text',
  '--cl-danger', '--cl-danger-bg', '--cl-danger-text',
  '--cl-info', '--cl-overlay', '--cl-shadow-1', '--cl-shadow-2',
]

const STATUS_CATEGORIES = ['todo', 'progress', 'done']
const ISSUE_TYPES = ['epic', 'story', 'task', 'bug', 'subtask']
const PRIORITIES = ['highest', 'high', 'medium', 'low', 'lowest']
const PALETTE_SIZE = 12
const AVATAR_SIZE = 10

const newThemeTokens = [
  '--cl-shadow-3',
  '--cl-shadow-focus',
  ...STATUS_CATEGORIES.flatMap((c) => [`--cl-cat-${c}`, `--cl-cat-${c}-bg`, `--cl-cat-${c}-text`]),
  ...ISSUE_TYPES.flatMap((t) => [`--cl-type-${t}`, `--cl-type-${t}-bg`, `--cl-type-${t}-text`]),
  ...PRIORITIES.map((p) => `--cl-prio-${p}`),
]

// Theme-independent palettes live only in :root (same in both themes).
const paletteNames = Array.from({ length: PALETTE_SIZE }, (_, i) => i + 1)
const avatarNames = Array.from({ length: AVATAR_SIZE }, (_, i) => i + 1)

// Existing tokens that fail the rules. Their values are kept as they are;
// the test asserts they still fail so the entry is removed once they are fixed.
const KNOWN_DEBT = new Set<string>([])

type Theme = 'light' | 'dark'

function valueOf(theme: Theme, name: string): string {
  const value = (theme === 'dark' ? dark : root).get(name) ?? root.get(name)
  if (value === undefined) throw new Error(`token not declared: ${name}`)
  if (!/^#[0-9A-Fa-f]{6}$/.test(value)) {
    throw new Error(`${name} is not a #RRGGBB color: ${value}`)
  }
  return value
}

function luminance(hex: string): number {
  const channels = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2]
}

// WCAG 2.1 contrast ratio (relative luminance based).
function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

describe('tokens.css and tokens.ts parity', () => {
  it('declares every theme-dependent token in both :root and the dark block', () => {
    const all = [...existingThemeTokens, ...newThemeTokens]
    const missing = all.flatMap((name) => {
      const out: string[] = []
      if (!root.has(name)) out.push(`:root ${name}`)
      if (!dark.has(name)) out.push(`dark ${name}`)
      return out
    })
    expect(missing).toEqual([])
  })

  it.each(['light', 'dark'] as const)('TS mirror (%s) has the same keys as tokens.css', (theme) => {
    expect(Object.keys(themeTokens[theme]).sort()).toEqual([...newThemeTokens].sort())
  })

  it.each(['light', 'dark'] as const)('TS mirror (%s) has the same values as tokens.css', (theme) => {
    const css = theme === 'dark' ? dark : root
    const mismatches = Object.entries(themeTokens[theme]).filter(
      ([name, value]) => css.get(name) !== value,
    )
    expect(mismatches).toEqual([])
  })

  it('does not declare theme-independent palettes in the dark block', () => {
    const leaked = [
      ...paletteNames.flatMap((i) => [`--cl-palette-${i}-bg`, `--cl-palette-${i}-text`]),
      ...avatarNames.flatMap((i) => [`--cl-avatar-${i}`, `--cl-avatar-${i}-text`]),
    ].filter((name) => dark.has(name))
    expect(leaked).toEqual([])
  })
})

interface ContrastCase {
  theme: Theme
  fg: string
  bg: string
  min: number
}

function buildContrastCases(): ContrastCase[] {
  const cases: ContrastCase[] = []
  const themes: Theme[] = ['light', 'dark']
  for (const theme of themes) {
    // Text on surfaces (AA text 4.5:1).
    for (const surface of ['--cl-bg-page', '--cl-bg-surface', '--cl-bg-sunken']) {
      cases.push({ theme, fg: '--cl-text', bg: surface, min: 4.5 })
      cases.push({ theme, fg: '--cl-text-subtle', bg: surface, min: 4.5 })
    }
    // Semantic and brand text on their tinted backgrounds.
    for (const kind of ['success', 'warning', 'danger']) {
      cases.push({ theme, fg: `--cl-${kind}-text`, bg: `--cl-${kind}-bg`, min: 4.5 })
    }
    cases.push({ theme, fg: '--cl-brand', bg: '--cl-brand-subtle', min: 4.5 })
    // UI components on surface (3:1).
    cases.push({ theme, fg: '--cl-brand', bg: '--cl-bg-surface', min: 3 })
    cases.push({ theme, fg: '--cl-border-strong', bg: '--cl-bg-surface', min: 3 })
    // Status, type and priority: text on tint (4.5), accent on surface (3).
    for (const c of STATUS_CATEGORIES) {
      cases.push({ theme, fg: `--cl-cat-${c}-text`, bg: `--cl-cat-${c}-bg`, min: 4.5 })
      cases.push({ theme, fg: `--cl-cat-${c}`, bg: '--cl-bg-surface', min: 3 })
    }
    for (const t of ISSUE_TYPES) {
      cases.push({ theme, fg: `--cl-type-${t}-text`, bg: `--cl-type-${t}-bg`, min: 4.5 })
      cases.push({ theme, fg: `--cl-type-${t}`, bg: '--cl-bg-surface', min: 3 })
    }
    for (const p of PRIORITIES) {
      cases.push({ theme, fg: `--cl-prio-${p}`, bg: '--cl-bg-surface', min: 3 })
    }
    // Shared palettes and avatars (checked in both themes).
    for (const i of paletteNames) {
      cases.push({ theme, fg: `--cl-palette-${i}-text`, bg: `--cl-palette-${i}-bg`, min: 4.5 })
    }
    for (const i of avatarNames) {
      cases.push({ theme, fg: `--cl-avatar-${i}-text`, bg: `--cl-avatar-${i}`, min: 4.5 })
    }
  }
  return cases
}

describe('WCAG contrast of declared pairs', () => {
  it.each(buildContrastCases())(
    '$theme: $fg on $bg >= $min',
    ({ theme, fg, bg, min }) => {
      const ratio = contrast(valueOf(theme, fg), valueOf(theme, bg))
      const key = `${theme}:${fg}/${bg}`
      if (KNOWN_DEBT.has(key)) {
        expect(ratio, `${key} is listed as known debt; remove it from KNOWN_DEBT once fixed`).toBeLessThan(min)
        return
      }
      expect(ratio, key).toBeGreaterThanOrEqual(min)
    },
  )

  it('computes the WCAG reference ratio for black on white', () => {
    expect(contrast('#000000', '#FFFFFF')).toBeCloseTo(21, 1)
  })
})

describe('layout, spacing, z-index and density tokens', () => {
  it('declares layout tokens', () => {
    expect(root.get('--cl-topbar-h')).toBe('56px')
    expect(root.get('--cl-sidebar-w')).toBe('240px')
    expect(root.get('--cl-sidebar-w-collapsed')).toBe('64px')
    expect(root.get('--cl-content-max')).toBe('1200px')
    expect(root.get('--cl-content-wide')).toBe('1680px')
    expect(root.get('--cl-gutter')).toBe('24px')
  })

  it('lowers the gutter to 16px below 768px', () => {
    expect(mobileGutter).not.toBeNull()
    expect(parseDeclarations(mobileGutter?.[1] ?? '').get('--cl-gutter')).toBe('16px')
  })

  it('declares the z-index scale in ascending order', () => {
    const order = ['--cl-z-header', '--cl-z-dropdown', '--cl-z-modal', '--cl-z-toast', '--cl-z-palette']
    const values = order.map((name) => Number(root.get(name)))
    expect(values.every((v) => Number.isFinite(v))).toBe(true)
    expect([...values].sort((a, b) => a - b)).toEqual(values)
    expect(new Set(values).size).toBe(values.length)
  })

  it('declares the new spacing steps', () => {
    expect(root.get('--cl-space-10')).toBe('40px')
    expect(root.get('--cl-space-12')).toBe('48px')
    expect(root.get('--cl-space-16')).toBe('64px')
  })

  it('declares density row heights', () => {
    expect(root.get('--cl-row-h')).toBe('40px')
    expect(root.get('--cl-row-h-compact')).toBe('32px')
  })

  it('declares the status, type, priority, palette and avatar tokens in :root', () => {
    const names = [
      ...paletteNames.flatMap((i) => [`--cl-palette-${i}-bg`, `--cl-palette-${i}-text`]),
      ...avatarNames.flatMap((i) => [`--cl-avatar-${i}`, `--cl-avatar-${i}-text`]),
    ]
    expect(names.filter((n) => !root.has(n))).toEqual([])
  })
})
