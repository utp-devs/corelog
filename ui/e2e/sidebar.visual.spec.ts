import { expect, test, type Page } from '@playwright/test'

import { login, waitForImages } from './helpers.ts'

/** Enlace de marca del sidebar (banner o icono): tamano fijo, independiente de los datos. */
const brandHeader = (page: Page) => page.locator('aside a[aria-label="CoreLog"]').first()

test.describe('sidebar de la aplicacion', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await waitForImages(page)
  })

  test('expandido muestra el banner completo', async ({ page }) => {
    const sider = page.locator('aside').first()
    const banner = sider.getByRole('img', { name: 'CoreLog' })

    await expect(banner).toBeVisible()
    await expect(banner).toHaveAttribute('src', '/brand/corelog-banner-480.png')

    // Guarda explicita de tamano: una captura con tolerancia no detecta cambios pequenos.
    const box = await banner.boundingBox()
    expect(box?.width).toBeGreaterThan(0)
    expect(box!.width).toBeLessThanOrEqual(160)

    // Solo el encabezado de marca: el alto del sidebar depende del contenido de la pagina.
    await expect(brandHeader(page)).toHaveScreenshot('sidebar-brand-expanded.png', { maxDiffPixels: 100 })
  })

  test('colapsado muestra solo el icono', async ({ page }) => {
    await page.locator('aside .ant-layout-sider-trigger').click()
    await expect(page.locator('aside').first()).toHaveClass(/ant-layout-sider-collapsed/)

    const icon = page.locator('aside').first().locator('img[src*="corelog-icon-64"]')
    await expect(icon).toBeVisible()
    await expect(page.locator('aside').first().getByRole('img', { name: 'CoreLog' })).toHaveCount(0)

    await expect(brandHeader(page)).toHaveScreenshot('sidebar-brand-collapsed.png', { maxDiffPixels: 100 })
  })
})
