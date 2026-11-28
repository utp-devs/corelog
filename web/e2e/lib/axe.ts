import AxeBuilder from '@axe-core/playwright';
import { expect, type Page } from '@playwright/test';

import { AXE_EXCLUSIONS, AXE_RULE_EXCLUSIONS } from './axe-exclusions.ts';

/** WCAG 2.0/2.1 level A and AA rules plus axe best practices (includes rendered color contrast). */
const TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'best-practice'];

export interface AxeOptions {
  /** Name of the screen or state, shown in the failure message. */
  readonly name: string;
  /** Extra selectors left out of this scan (the shared list lives in axe-exclusions.ts). */
  readonly exclude?: readonly string[];
  /** Restricts the scan to a subtree (for example an open dialog). */
  readonly include?: string;
}

/** Runs axe and returns a readable summary of every violation (empty string when clean). */
export async function axeSummary(page: Page, options: AxeOptions): Promise<string> {
  let builder = new AxeBuilder({ page }).withTags(TAGS);
  if (options.include) builder = builder.include(options.include);
  for (const selector of [...AXE_EXCLUSIONS.map((e) => e.selector), ...(options.exclude ?? [])]) {
    builder = builder.exclude(selector);
  }
  const results = await builder.analyze();
  const violations = results.violations
    .map((violation) => ({
      ...violation,
      nodes: violation.nodes.filter(
        (node) =>
          !AXE_RULE_EXCLUSIONS.some(
            (waiver) =>
              waiver.rule === violation.id &&
              waiver.selector
                .split(',')
                .some((part) => node.target.join(' ').startsWith(part.trim())),
          ),
      ),
    }))
    .filter((violation) => violation.nodes.length > 0);
  const lines = violations.map((violation) => {
    const nodes = violation.nodes
      .slice(0, 5)
      .map((node) => {
        const detail = (node.any[0]?.message ?? node.failureSummary ?? '').split('\n')[0];
        return `      ${node.target.join(' ')} | ${detail}`;
      })
      .join('\n');
    const more =
      violation.nodes.length > 5 ? `\n      ... +${violation.nodes.length - 5} more` : '';
    return `  [${violation.impact ?? 'n/a'}] ${violation.id}: ${violation.help} (${violation.nodes.length} nodes)\n${nodes}${more}`;
  });
  return lines.length > 0 ? `axe violations on "${options.name}":\n${lines.join('\n')}` : '';
}

/** Runs axe on the current page state and fails with the summary of every violation. */
export async function expectNoAxeViolations(page: Page, options: AxeOptions): Promise<void> {
  const summary = await axeSummary(page, options);
  expect(summary, summary).toBe('');
}
