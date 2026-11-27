import { test } from '@playwright/test';

/**
 * Visual baselines are generated and compared only on Linux (official Playwright container).
 * Call inside a describe block whose title contains `@visual`.
 */
export function skipVisualOutsideLinux(): void {
  test.skip(process.platform !== 'linux', 'visual baselines are Linux-only');
}
