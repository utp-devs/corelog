import type { Page } from '@playwright/test'

import { readE2eEnv } from './env.ts'
import { expect } from '../fixtures/test.ts'

/** Logs in through the login form (no storage state). Resolves when the app left /login. */
export async function loginViaUi(page: Page, email: string, password = readE2eEnv().password ?? ''): Promise<void> {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(email)
  await page.locator('input[type="password"]').fill(password)
  await page.getByRole('button', { name: 'Iniciar sesión' }).click()
  await expect(page).not.toHaveURL(/\/login/)
}
