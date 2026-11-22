import { defineConfig, devices } from '@playwright/test'

// Por defecto los tests corren contra el stack Docker (docker-compose.local.yaml):
// la API solo acepta CORS desde http://localhost:5173, asi que un dev server en otro
// puerto no podria iniciar sesion. Para apuntar a otra URL usa E2E_BASE_URL.
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:5173'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  outputDir: 'test-results',
  timeout: 30_000,
  expect: {
    toHaveScreenshot: {
      // Pequena tolerancia para antialiasing de fuentes entre ejecuciones.
      maxDiffPixelRatio: 0.01,
      animations: 'disabled',
    },
  },
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1280, height: 800 } },
    },
  ],
})
