import type { Page } from '@playwright/test';

/** Time given to the app's own readiness marker before falling back to network idle. */
const READY_MARKER_TIMEOUT_MS = 2_000;
const NETWORK_IDLE_TIMEOUT_MS = 5_000;

/** Waits until every image of the page has finished loading (avoids captures taken mid-load). */
export async function waitForImages(page: Page): Promise<void> {
  await page.waitForFunction(() => Array.from(document.images).every((img) => img.complete));
}

/**
 * Waits for the page to report ready (`data-ready="true"`, set by PageContainer once its data resolved).
 * Returns false when the marker never appears; the caller then relies on network idle instead.
 */
export async function waitForReady(page: Page): Promise<boolean> {
  try {
    await page
      .locator('[data-ready="true"]')
      .first()
      .waitFor({ state: 'attached', timeout: READY_MARKER_TIMEOUT_MS });
    return true;
  } catch {
    await page
      .waitForLoadState('networkidle', { timeout: NETWORK_IDLE_TIMEOUT_MS })
      .catch(() => undefined);
    return false;
  }
}
