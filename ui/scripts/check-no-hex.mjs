#!/usr/bin/env node
// check-no-hex.mjs - guard against hardcoded colors and inline styles in ui/src.
//
// Scans ui/src/**/*.{ts,tsx,css} and reports:
//   - hex colors (#abc, #aabbcc, #aabbccdd)
//   - rgb(), rgba(), hsl(), hsla() literals
//   - style={{ ... }} unless the line or the previous line contains the
//     marker "/* runtime-style */" (for runtime-computed values only)
// Excluded: styles/tokens.css, shared/theme/tokens.ts, *.test.* / *.spec.*, e2e/.
//
// Baseline: ui/scripts/check-no-hex.baseline.json maps each file (path relative
// to ui/, POSIX separators) to the number of violations it is allowed to keep.
// The script fails when a file exceeds its baseline count; a file missing from
// the baseline has a baseline of 0, so new violations always fail.
// The baseline is expected to shrink to empty as the existing screens are redesigned.
//
// Usage (from ui/):
//   node scripts/check-no-hex.mjs                  check against the baseline
//   node scripts/check-no-hex.mjs --write-baseline regenerate the baseline (only with user approval)
// Wired as `npm run check:tokens`, and run by `npm run lint`.

import { existsSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { dirname, join, relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const uiDir = join(dirname(fileURLToPath(import.meta.url)), '..')
const srcDir = join(uiDir, 'src')
const baselinePath = join(uiDir, 'scripts', 'check-no-hex.baseline.json')

const EXTENSIONS = ['.ts', '.tsx', '.css']
const EXCLUDED_FILES = new Set(['src/styles/tokens.css', 'src/shared/theme/tokens.ts'])
const EXCLUDED_DIRS = new Set(['e2e', 'node_modules'])
const TEST_FILE = /\.(test|spec)\.(ts|tsx|css)$/
const RUNTIME_MARKER = '/* runtime-style */'

const rules = [
  {
    id: 'hex-color',
    pattern: /(?<![\w&])#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3})(?![\w-])/g,
    message: 'hex color: use a --cl-* token',
  },
  {
    id: 'rgb-hsl',
    pattern: /\b(?:rgba?|hsla?)\s*\(/gi,
    message: 'rgb/hsl color literal: use a --cl-* token',
  },
  {
    id: 'inline-style',
    pattern: /style=\{\{/g,
    message: 'inline style object: use a class or a CSS custom property (or mark /* runtime-style */)',
    allowed: (line, prev) => line.includes(RUNTIME_MARKER) || prev.includes(RUNTIME_MARKER),
  },
]

function toPosix(path) {
  return relative(uiDir, path).split(sep).join('/')
}

function walk(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (!EXCLUDED_DIRS.has(entry.name)) walk(full, out)
      continue
    }
    const ext = EXTENSIONS.find((e) => entry.name.endsWith(e))
    if (!ext || TEST_FILE.test(entry.name)) continue
    if (EXCLUDED_FILES.has(toPosix(full))) continue
    out.push(full)
  }
  return out
}

function scanFile(file) {
  const lines = readFileSync(file, 'utf8').split(/\r?\n/)
  const violations = []
  lines.forEach((line, index) => {
    const prev = index > 0 ? lines[index - 1] : ''
    for (const rule of rules) {
      const matches = [...line.matchAll(rule.pattern)]
      if (matches.length === 0) continue
      if (rule.allowed && rule.allowed(line, prev)) continue
      for (let n = 0; n < matches.length; n += 1) {
        violations.push({ line: index + 1, rule: rule.id, message: rule.message, text: line.trim() })
      }
    }
  })
  return violations
}

function loadBaseline() {
  if (!existsSync(baselinePath)) return {}
  const parsed = JSON.parse(readFileSync(baselinePath, 'utf8'))
  return parsed.files ?? {}
}

const writeMode = process.argv.includes('--write-baseline')
const files = walk(srcDir).sort()
const scanned = new Map()
for (const file of files) {
  const violations = scanFile(file)
  if (violations.length > 0) scanned.set(toPosix(file), violations)
}

if (writeMode) {
  const counts = Object.fromEntries([...scanned].map(([file, v]) => [file, v.length]))
  const payload = {
    note: 'Violations allowed per file while pages are migrated (the file is expected to shrink to empty as the screens are fixed). Regenerate only with: node scripts/check-no-hex.mjs --write-baseline',
    files: counts,
  }
  writeFileSync(baselinePath, `${JSON.stringify(payload, null, 2)}\n`)
  const total = Object.values(counts).reduce((sum, n) => sum + n, 0)
  console.log(`baseline written: ${Object.keys(counts).length} files, ${total} violations`)
  process.exit(0)
}

const baseline = loadBaseline()
const failures = []
let total = 0
for (const [file, violations] of scanned) {
  total += violations.length
  const allowed = baseline[file] ?? 0
  if (violations.length > allowed) {
    failures.push({ file, violations, allowed })
  }
}

const stale = Object.keys(baseline).filter((file) => (scanned.get(file)?.length ?? 0) < baseline[file])

for (const { file, violations, allowed } of failures) {
  console.error(`${file}: ${violations.length} violation(s), baseline allows ${allowed}`)
  for (const v of violations) {
    console.error(`  ${file}:${v.line} [${v.rule}] ${v.message}`)
    console.error(`    ${v.text}`)
  }
}

if (stale.length > 0) {
  console.log(`note: ${stale.length} baseline entr${stale.length === 1 ? 'y' : 'ies'} can be lowered (run --write-baseline after fixing)`)
}

const baselineTotal = Object.values(baseline).reduce((sum, n) => sum + n, 0)
console.log(
  `check-no-hex: ${files.length} files scanned, ${total} violation(s) found, ` +
    `baseline ${Object.keys(baseline).length} file(s) / ${baselineTotal} violation(s)`,
)

if (failures.length > 0) {
  console.error(`check-no-hex: FAILED in ${failures.length} file(s)`)
  process.exit(1)
}
console.log('check-no-hex: OK')
