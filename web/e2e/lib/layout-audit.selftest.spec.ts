import { expect, test } from '@playwright/test';

import { collectLayoutAudit, trackHygiene, type AuditOptions } from './layout-audit.ts';

// Self tests: pages built with page.setContent, no application or API needed.
// Runs in the `selftest` project (desktop viewport, no global setup).

/** Twelve 200px-tall cards in a 3-column grid that fills the 1440x900 desktop viewport. */
const FILLED_GRID = Array.from(
  { length: 12 },
  (_, index) =>
    `<article data-testid="card-${index}" style="background:#dbe7ff;border-radius:8px;padding:12px;height:200px;box-sizing:border-box">` +
    `<h2 style="margin:0 0 8px;font-size:16px">Card ${index + 1}</h2>` +
    `<p style="margin:0;font-size:14px">Some content that wraps across the card.</p></article>`,
).join('');

const GOOD_PAGE = `<main data-ready="true" style="box-sizing:border-box;padding:24px;min-height:100vh">
  <div data-page>
    <div data-card-grid style="display:grid;grid-template-columns:repeat(3,1fr);gap:16px">${FILLED_GRID}</div>
  </div>
</main>`;

const WIDE: AuditOptions = { template: 'wide', name: 'selftest' };

function pageHtml(main: string): string {
  return `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><style>
    body { margin: 0; font-family: sans-serif; }
  </style></head><body>${main}</body></html>`;
}

test.describe('layout audit self tests', () => {
  test('a well-filled page passes every check', async ({ page }) => {
    trackHygiene(page);
    await page.setContent(pageHtml(GOOD_PAGE));

    const result = await collectLayoutAudit(page, WIDE);

    expect(result.violations).toEqual([]);
    expect(result.metrics.viewport.width).toBe(1440);
    expect(result.metrics.coveragePct).not.toBeNull();
  });

  test('horizontal overflow is reported', async ({ page }) => {
    trackHygiene(page);
    const wide = GOOD_PAGE.replace(
      '<div data-page>',
      '<div data-page><div style="width:2000px;height:20px;background:#f00">wide</div>',
    );
    await page.setContent(pageHtml(wide));

    const result = await collectLayoutAudit(page, WIDE);

    expect(result.violations.some((v) => v.startsWith('horizontal overflow'))).toBe(true);
    expect(result.metrics.overflowPx).toBeGreaterThan(1);
  });

  test('a large empty area is reported as dead space', async ({ page }) => {
    trackHygiene(page);
    await page.setContent(
      pageHtml(`<main data-ready="true" style="box-sizing:border-box;padding:24px;min-height:100vh">
        <div data-page><div style="height:80px;background:#ccc">Header only</div></div>
      </main>`),
    );

    const result = await collectLayoutAudit(page, WIDE);

    expect(result.violations.some((v) => v.startsWith('dead space'))).toBe(true);
    expect(result.metrics.largestEmptyRect?.areaRatio ?? 0).toBeGreaterThan(0.3);
  });

  test('overlapping siblings are reported', async ({ page }) => {
    trackHygiene(page);
    const overlapping = GOOD_PAGE.replace(
      '<div data-page>',
      '<div data-page style="display:flex">' +
        '<div style="width:400px;height:200px;background:#eee">A</div>' +
        '<div style="width:400px;height:200px;margin-left:-200px;background:#999">B</div>' +
        '</div><div hidden>',
    );
    await page.setContent(pageHtml(overlapping));

    const result = await collectLayoutAudit(page, WIDE);

    expect(result.violations.some((v) => v.startsWith('overlap:'))).toBe(true);
    expect(result.metrics.overlapPairs).toBeGreaterThan(0);
  });

  test('SVG internals and table sections are not reported as overlap', async ({ page }) => {
    trackHygiene(page);
    const svgAndTable = GOOD_PAGE.replace(
      '<div data-page>',
      '<div data-page><svg width="120" height="120"><circle cx="50" cy="50" r="40"/><circle cx="70" cy="50" r="40"/></svg>' +
        '<table><colgroup><col/><col/></colgroup><thead><tr><th>A</th></tr></thead><tbody><tr><td>B</td></tr></tbody></table>',
    );
    await page.setContent(pageHtml(svgAndTable));

    const result = await collectLayoutAudit(page, WIDE);

    expect(result.violations.some((v) => v.startsWith('overlap'))).toBe(false);
    expect(result.metrics.overlapPairs).toBe(0);
  });

  test('clipped text without an ellipsis title is reported', async ({ page }) => {
    trackHygiene(page);
    const clipped = GOOD_PAGE.replace(
      '<div data-page>',
      '<div data-page><div style="width:120px;overflow:hidden;white-space:nowrap;background:#eee">' +
        'A very long label that cannot fit in its box</div>',
    );
    await page.setContent(pageHtml(clipped));

    const result = await collectLayoutAudit(page, WIDE);

    expect(result.violations.some((v) => v.startsWith('clipped text'))).toBe(true);
    expect(result.metrics.clippedText).toBeGreaterThan(0);
  });

  test('a console error is reported by the hygiene check', async ({ page }) => {
    trackHygiene(page);
    await page.setContent(pageHtml(GOOD_PAGE));
    await page.evaluate(() => console.error('selftest console error'));

    const result = await collectLayoutAudit(page, WIDE);

    expect(result.violations.some((v) => v.startsWith('console errors'))).toBe(true);
    expect(result.metrics.consoleErrors).toBe(1);
  });

  test('wrapped inline text fragments are not reported as overlap', async ({ page }) => {
    trackHygiene(page);
    const wrapped = GOOD_PAGE.replace(
      '<div data-page>',
      '<div data-page><p style="width:160px;margin:0"><a href="#">Crea tu primer proyecto</a>' +
        '<span> para organizar el trabajo en un espacio propio y largo</span></p>',
    );
    await page.setContent(pageHtml(wrapped));
    const result = await collectLayoutAudit(page, WIDE);
    expect(result.metrics.overlapPairs).toBe(0);
  });

  test('a declared browser resource error is not a console error', async ({ page }) => {
    trackHygiene(page);
    await page.setContent(pageHtml(GOOD_PAGE));
    await page.evaluate(() =>
      console.error(
        'Failed to load resource: the server responded with a status of 403 (Forbidden)',
      ),
    );
    const declared = await collectLayoutAudit(page, {
      ...WIDE,
      expectedFailures: [{ status: 403, urlPattern: /\/api\/v1\// }],
    });
    expect(declared.metrics.consoleErrors).toBe(0);
    const undeclared = await collectLayoutAudit(page, WIDE);
    expect(undeclared.metrics.consoleErrors).toBe(1);
  });

  test('visually hidden text is not reported as clipped', async ({ page }) => {
    trackHygiene(page);
    const srOnly = GOOD_PAGE.replace(
      '<div data-page>',
      '<div data-page><label>Clave <span style="position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0"> (obligatorio)</span></label>',
    );
    await page.setContent(pageHtml(srOnly));
    const result = await collectLayoutAudit(page, WIDE);
    expect(result.metrics.clippedText).toBe(0);
  });
});
