import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve, sep } from 'node:path'

import { describe, expect, it } from 'vitest'

/**
 * Layering guard: code under shared/ must not depend on the adapters, the app shell or the core.
 * There are no exceptions: shared vocabulary lives in shared/types.
 */
const srcDir = resolve(process.cwd(), 'src')
const sharedDir = join(srcDir, 'shared')
const FORBIDDEN_LAYERS = ['adapters', 'app', 'core'].map((name) => join(srcDir, name))
const TEST_FILE = /\.test\.tsx?$/
const IMPORT_PATTERN = /^\s*(import|export)\s+(type\s+)?(?:[^'"`;]*?\s+from\s+)?['"]([^'"]+)['"]/gm
const DYNAMIC_IMPORT_PATTERN = /import\(\s*['"]([^'"]+)['"]\s*\)/g

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
  const specifiers: string[] = []
  for (const match of text.matchAll(IMPORT_PATTERN)) specifiers.push(match[3])
  for (const match of text.matchAll(DYNAMIC_IMPORT_PATTERN)) specifiers.push(match[1])
  return specifiers.filter((specifier) => resolvesToForbiddenLayer(file, specifier)).map((specifier) => `${rel}: ${specifier}`)
}

describe('layering guard', () => {
  it('nothing under ui/src/shared imports from adapters, app or core', () => {
    const offenders = walk(sharedDir).flatMap(violationsOf)
    expect(offenders).toEqual([])
  })
})
