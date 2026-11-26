import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve, sep } from 'node:path'

import { describe, expect, it } from 'vitest'

/**
 * Layering guard: code under shared/ must not depend on the adapters, the app shell or the core.
 * Only type-only imports of a core contract are tolerated, and only from the files listed below.
 */
const srcDir = resolve(process.cwd(), 'src')
const sharedDir = join(srcDir, 'shared')
const FORBIDDEN_LAYERS = ['adapters', 'app', 'core'].map((name) => join(srcDir, name))
const TEST_FILE = /\.test\.tsx?$/
const IMPORT_PATTERN = /^\s*(import|export)\s+(type\s+)?(?:[^'"`;]*?\s+from\s+)?['"]([^'"]+)['"]/gm
const DYNAMIC_IMPORT_PATTERN = /import\(\s*['"]([^'"]+)['"]\s*\)/g

/** Type-only contracts of core that shared UI components still reference (move them when the owner allows). */
const TYPE_ONLY_CORE_EXCEPTIONS = new Set([
  'shared/ui/CommandPalette/CommandPalette.tsx',
  'shared/ui/CommandPalette/groups.ts',
  'shared/ui/CommandPalette/recent.ts',
  'shared/ui/IssueKey/IssueKey.tsx',
  'shared/ui/IssueTypeIcon.tsx',
  'shared/ui/PriorityTag.tsx',
  'shared/ui/StatusTag.tsx',
])

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) {
      walk(full, out)
    } else if (/\.tsx?$/.test(name) && !TEST_FILE.test(name)) {
      out.push(full)
    }
  }
  return out
}

function resolvesToForbiddenLayer(file: string, specifier: string): boolean {
  let target: string
  if (specifier.startsWith('@/')) {
    target = join(srcDir, specifier.slice(2))
  } else if (specifier.startsWith('.')) {
    target = resolve(dirname(file), specifier)
  } else {
    return false
  }
  return FORBIDDEN_LAYERS.some((layer) => target === layer || target.startsWith(layer + sep))
}

/** Lists "relative/file: import" entries that break the layering rule. */
function violationsOf(file: string): string[] {
  const rel = relative(srcDir, file).split(sep).join('/')
  const text = readFileSync(file, 'utf8')
  const found: string[] = []
  const specifiers: { specifier: string; typeOnly: boolean }[] = []
  for (const match of text.matchAll(IMPORT_PATTERN)) {
    specifiers.push({ specifier: match[3], typeOnly: match[2] !== undefined })
  }
  for (const match of text.matchAll(DYNAMIC_IMPORT_PATTERN)) {
    specifiers.push({ specifier: match[1], typeOnly: false })
  }
  for (const { specifier, typeOnly } of specifiers) {
    if (!resolvesToForbiddenLayer(file, specifier)) continue
    const isCore = /(^@\/core\/)|(\/core\/)/.test(specifier)
    if (typeOnly && isCore && TYPE_ONLY_CORE_EXCEPTIONS.has(rel)) continue
    found.push(`${rel}: ${specifier}`)
  }
  return found
}

describe('layering guard', () => {
  it('nothing under ui/src/shared imports from adapters, app or core (except listed type-only contracts)', () => {
    const offenders = walk(sharedDir).flatMap(violationsOf)
    expect(offenders).toEqual([])
  })

  it('every exception names a file that exists', () => {
    const existing = new Set(walk(sharedDir).map((file) => relative(srcDir, file).split(sep).join('/')))
    for (const rel of TYPE_ONLY_CORE_EXCEPTIONS) {
      expect(existing.has(rel), `${rel} is listed but does not exist`).toBe(true)
    }
  })
})
