import { expect, test } from '@playwright/test'

import { waitForImages } from './helpers.ts'

test.describe('pantalla de login', () => {
  test('muestra el banner de CoreLog centrado sobre el formulario', async ({ page }) => {
    await page.goto('/login')
    await waitForImages(page)

    const banner = page.getByRole('img', { name: 'CoreLog' })
    await expect(banner).toBeVisible()
    await expect(banner).toHaveAttribute('src', '/brand/corelog-banner-480.png')

    await expect(page).toHaveScreenshot('login.png', { fullPage: true })
  })
})
