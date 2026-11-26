import { defineConfig, devices } from '@playwright/test'

import { readE2eEnv } from './e2e/lib/env.ts'

// Settings: process environment first, then ui/.env.e2e (git-ignored), then defaults.
// The API only accepts CORS from http://localhost:5173, so the default target is the Docker stack.
const env = readE2eEnv()

const desktop = { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } }

// Specs that test the audit itself: no stack and no login needed.
const SELFTEST = /layout-audit\.selftest\.spec\.ts$/
const SETUP = /global-setup\.ts$/

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
      // Logs every role in and writes ui/playwright/.auth/<role>.json. Viewport projects depend on it.
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
      name: 'mobile',
      testIgnore: [SELFTEST, SETUP],
      dependencies: ['setup'],
      use: { ...devices['Desktop Chrome'], viewport: { width: 390, height: 844 }, hasTouch: true },
    },
  ],
})
