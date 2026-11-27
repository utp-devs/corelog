import { defineConfig, devices } from '@playwright/test';

import { readE2eEnv } from './e2e/lib/env.ts';

// Settings: process environment first, then the git-ignored env file (see e2e/lib/env.ts), then defaults.
// The Angular build is served from one origin (npm run serve:dist), so the default target is that origin.
const env = readE2eEnv();

const desktop = { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } };

// Specs that test the audit itself: no stack and no login needed.
const SELFTEST = /layout-audit\.selftest\.spec\.ts$/;
const SETUP = /global-setup\.ts$/;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  outputDir: 'test-results',
  globalTeardown: './e2e/global-teardown.ts',
  timeout: 60_000,
  expect: {
    // Dev server modules compile on first load: the default 5 s is too short for the first visit of a screen.
    timeout: 15_000,
    toHaveScreenshot: {
      // Small tolerance for font antialiasing between runs of the same platform.
      maxDiffPixelRatio: 0.01,
      animations: 'disabled',
    },
  },
  use: {
    baseURL: env.baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      // Logs every role in and writes web/playwright/.auth/<role>.json. Viewport projects depend on it.
      name: 'setup',
      testMatch: SETUP,
    },
    {
      name: 'selftest',
      testMatch: SELFTEST,
      use: { ...desktop },
    },
    {
      name: 'desktop',
      testIgnore: [SELFTEST, SETUP],
      dependencies: ['setup'],
      use: { ...desktop },
    },
    {
      name: 'tablet',
      testIgnore: [SELFTEST, SETUP],
      dependencies: ['setup'],
      use: { ...devices['Desktop Chrome'], viewport: { width: 820, height: 1180 } },
    },
    {
      // Dark theme runs: same viewports, the dark color scheme and the stored dark theme (see fixtures/test.ts).
      name: 'desktop-dark',
      testIgnore: [SELFTEST, SETUP],
      dependencies: ['setup'],
      use: { ...desktop, colorScheme: 'dark' },
    },
    {
      name: 'tablet-dark',
      testIgnore: [SELFTEST, SETUP],
      dependencies: ['setup'],
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 820, height: 1180 },
        colorScheme: 'dark',
      },
    },
    {
      name: 'mobile-dark',
      testIgnore: [SELFTEST, SETUP],
      dependencies: ['setup'],
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 390, height: 844 },
        hasTouch: true,
        colorScheme: 'dark',
      },
    },
    {
      name: 'mobile',
      testIgnore: [SELFTEST, SETUP],
      dependencies: ['setup'],
      use: { ...devices['Desktop Chrome'], viewport: { width: 390, height: 844 }, hasTouch: true },
    },
  ],
});
