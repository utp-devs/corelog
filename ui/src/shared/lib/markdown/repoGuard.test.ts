import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, resolve, sep } from 'node:path'

import { describe, expect, it } from 'vitest'

const srcDir = resolve(process.cwd(), 'src')
const FORBIDDEN = 'dangerouslySetInnerHTML'
const TEST_FILE = /\.test\.tsx?$/

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) {
      walk(full, out)
    } else if (name.endsWith('.tsx') && !TEST_FILE.test(name)) {
      out.push(full)
    }
  }
  return out
}

describe('repo guard', () => {
  it('no non-test .tsx file under ui/src uses dangerouslySetInnerHTML', () => {
    const offenders = walk(srcDir)
      .filter((file) => readFileSync(file, 'utf8').includes(FORBIDDEN))
      .map((file) => relative(srcDir, file).split(sep).join('/'))
    expect(offenders).toEqual([])
  })
})
