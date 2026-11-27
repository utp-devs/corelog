/**
 * Known-violations baseline for the smoke journey (web/e2e/known-violations.json).
 *
 * Shape: { "<screen>|<viewport>": ["<rule id>", ...] } where the rule id is the audit rule of a
 * violation ("dead space", "overlap", "clipped text", "tap targets", "width", "card heights",
 * "horizontal overflow", "console errors", "unexpected responses", ...).
 *
 * Behaviour of the smoke test for every screen and viewport:
 *  - a violation whose rule is listed for that screen|viewport is "known": it is printed in the
 *    summary and attached to the JSON report, and does NOT fail the test;
 *  - a violation whose rule is not listed is "fresh": it FAILS the test;
 *  - a listed rule that no longer occurs is "fixed": reported, does NOT fail.
 *  - "page errors" (uncaught exceptions) and "server errors" (5xx) are never baselined: always fresh.
 *
 * This baseline is expected to shrink to empty as the screens are fixed: once every screen complies
 * with the layout rules, the file becomes `{}` and every violation fails the suite. Until then, regenerate it only after a
 * deliberate review of the violations: `E2E_UPDATE_KNOWN=1 npx playwright test` (all projects)
 * merges what each role and viewport observed (see global-teardown.ts). Never regenerate it to hide
 * a regression introduced by a change.
 */
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const KNOWN_FILE = fileURLToPath(new URL('../known-violations.json', import.meta.url));
export const OBSERVED_DIR = fileURLToPath(
  new URL('../../test-results/journeys/observed/', import.meta.url),
);

/** Rules that can never be baselined. */
const HARD_RULES: ReadonlySet<string> = new Set(['page errors', 'server errors']);

export type KnownBaseline = Record<string, string[]>;

export interface Classification {
  readonly known: string[];
  readonly fresh: string[];
  readonly fixed: string[];
}

/** Rule id of a violation message: the text before the first colon, without the "(column N)" suffix. */
export function ruleIdOf(violation: string): string {
  const head = violation.split(':')[0] ?? violation;
  return head.replace(/\s*\(column \d+\)/, '').trim();
}

export function loadKnownViolations(file: string = KNOWN_FILE): KnownBaseline {
  if (!existsSync(file)) return {};
  const parsed: unknown = JSON.parse(readFileSync(file, 'utf8'));
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error(`${file} must be a JSON object of "<screen>|<viewport>" to rule id arrays`);
  }
  const baseline: KnownBaseline = {};
  for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
    if (!Array.isArray(value) || !value.every((item) => typeof item === 'string')) {
      throw new Error(`${file}: entry "${key}" must be an array of rule ids`);
    }
    baseline[key] = value as string[];
  }
  return baseline;
}

/** Splits one screen's violations against the baseline entry for the same screen|viewport key. */
export function classifyViolations(
  key: string,
  violations: readonly string[],
  baseline: KnownBaseline,
): Classification {
  const allowed = new Set(baseline[key] ?? []);
  const observed = new Set(violations.map(ruleIdOf));
  const known: string[] = [];
  const fresh: string[] = [];
  for (const violation of violations) {
    const rule = ruleIdOf(violation);
    if (allowed.has(rule) && !HARD_RULES.has(rule)) known.push(violation);
    else fresh.push(violation);
  }
  const fixed = [...allowed].filter((rule) => !observed.has(rule));
  return { known, fresh, fixed };
}

/** Writes what one role/viewport run observed (update mode only). */
export function writeObserved(name: string, observed: Record<string, string[]>): void {
  mkdirSync(OBSERVED_DIR, { recursive: true });
  writeFileSync(path.join(OBSERVED_DIR, `${name}.json`), JSON.stringify(observed, null, 2));
}

/** Unions every observed file into a sorted baseline. */
export function mergeObserved(dir: string = OBSERVED_DIR): KnownBaseline {
  const merged = new Map<string, Set<string>>();
  if (existsSync(dir)) {
    for (const file of readdirSync(dir).filter((name) => name.endsWith('.json'))) {
      const parsed = JSON.parse(readFileSync(path.join(dir, file), 'utf8')) as Record<
        string,
        string[]
      >;
      for (const [key, rules] of Object.entries(parsed)) {
        const set = merged.get(key) ?? new Set<string>();
        for (const rule of rules) set.add(rule);
        merged.set(key, set);
      }
    }
  }
  const sorted: KnownBaseline = {};
  for (const key of [...merged.keys()].sort()) {
    sorted[key] = [...(merged.get(key) ?? [])].sort();
  }
  return sorted;
}

export function writeKnownViolations(baseline: KnownBaseline, file: string = KNOWN_FILE): void {
  writeFileSync(file, `${JSON.stringify(baseline, null, 2)}\n`);
}
