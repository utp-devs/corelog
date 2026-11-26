import { skipVisualOutsideLinux } from '../../lib/visual.ts'
import { waitForImages } from '../../lib/wait.ts'
import { expect, test, type OpenAsRole } from '../../fixtures/test.ts'
import type { Page } from '@playwright/test'

/** Enlace de marca del sidebar (banner o icono): tamano fijo, independiente de los datos. */
const brandHeader = (page: Page) => page.locator('aside a[aria-label="CoreLog"]').first()

async function openShell(as: OpenAsRole): Promise<Page> {
  const page = await as('admin')
  await page.goto('/issues')
  await waitForImages(page)
  return page
}

test.describe('sidebar de la aplicacion', () => {
  // The sidebar is expanded only at desktop widths; narrower projects use the drawer instead.
  test.skip(({ viewport }) => (viewport?.width ?? 0) < 1280, 'sidebar expandido solo en escritorio')

  test('expandido muestra el banner completo', async ({ as }) => {
    const page = await openShell(as)
    const banner = page.locator('aside').first().getByRole('img', { name: 'CoreLog' })

    await expect(banner).toBeVisible()
    await expect(banner).toHaveAttribute('src', '/brand/corelog-banner-480.png')

    // Guarda explicita de tamano: una captura con tolerancia no detecta cambios pequenos.
    const box = await banner.boundingBox()
    expect(box?.width).toBeGreaterThan(0)
    expect(box!.width).toBeLessThanOrEqual(160)
  })

  test('colapsado muestra solo el icono', async ({ as }) => {
    const page = await openShell(as)
    await page.getByRole('button', { name: 'Contraer navegacion' }).click()
    await expect(page.locator('aside').first()).toHaveClass(/ant-layout-sider-collapsed/)

    const icon = page.locator('aside').first().locator('img[src*="corelog-icon-64"]')
    await expect(icon).toBeVisible()
    await expect(page.locator('aside').first().getByRole('img', { name: 'CoreLog' })).toHaveCount(0)
  })

  test.describe('@visual capturas del encabezado de marca', () => {
    skipVisualOutsideLinux()

    test('expandido: banner', async ({ as }) => {
      const page = await openShell(as)
      await expect(brandHeader(page)).toHaveScreenshot('sidebar-brand-expanded.png', { maxDiffPixels: 100 })
    })

    test('colapsado: icono', async ({ as }) => {
      const page = await openShell(as)
      await page.getByRole('button', { name: 'Contraer navegacion' }).click()
      await expect(page.locator('aside').first()).toHaveClass(/ant-layout-sider-collapsed/)
      await expect(brandHeader(page)).toHaveScreenshot('sidebar-brand-collapsed.png', { maxDiffPixels: 100 })
    })
  })
})
