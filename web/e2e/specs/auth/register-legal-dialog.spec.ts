import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { fillRegisterSteps } from '../../lib/register.ts';

const VIEWPORTS = [
  { width: 1920, height: 917 },
  { width: 1280, height: 720 },
  { width: 390, height: 844 },
] as const;

const ACCOUNT = {
  firstName: 'Ana',
  lastName: 'Torres',
  phone: '+51 912 345 678',
  email: 'ana.legal@corelog.test',
  password: 'Secreta-12345',
};

/** The dialog mask covers the whole viewport and the panel is centered and fully inside it. */
async function expectCoversViewport(page: Page): Promise<void> {
  const viewport = page.viewportSize();
  // The mask is the fixed-position ancestor of the dialog.
  const mask = await page.getByRole('dialog').evaluate((el) => {
    let node: HTMLElement | null = el.parentElement;
    while (node && getComputedStyle(node).position !== 'fixed') node = node.parentElement;
    if (!node) return null;
    const box = node.getBoundingClientRect();
    return { x: box.x, y: box.y, width: box.width, height: box.height };
  });
  const panel = await page.getByRole('dialog').boundingBox();
  if (!viewport || !mask || !panel) throw new Error('dialog geometry missing');
  expect(Math.abs(mask.x)).toBeLessThanOrEqual(1);
  expect(Math.abs(mask.y)).toBeLessThanOrEqual(1);
  expect(Math.abs(mask.width - viewport.width)).toBeLessThanOrEqual(1);
  expect(Math.abs(mask.height - viewport.height)).toBeLessThanOrEqual(1);
  expect(panel.x).toBeGreaterThanOrEqual(0);
  expect(panel.y).toBeGreaterThanOrEqual(0);
  expect(panel.x + panel.width).toBeLessThanOrEqual(viewport.width + 1);
  expect(panel.y + panel.height).toBeLessThanOrEqual(viewport.height + 3);
  expect(Math.abs(panel.x + panel.width / 2 - viewport.width / 2)).toBeLessThanOrEqual(2);
}

test.describe('documentos legales en el último paso del registro', () => {
  for (const viewport of VIEWPORTS) {
    test(`la ventana cubre toda la pantalla (${viewport.width}x${viewport.height})`, async ({
      page,
    }) => {
      await page.setViewportSize(viewport);
      await page.goto(appPath('/register'));
      await fillRegisterSteps(page, ACCOUNT);

      // The step animation must leave no transform behind: it would become the containing block of fixed overlays.
      await page.waitForTimeout(400);
      const transform = await page
        .getByTestId('register-step-3')
        .evaluate((el) => getComputedStyle(el).transform);
      expect(transform).toBe('none');

      const link = page.getByRole('button', { name: 'Política de privacidad', exact: true });
      await link.first().click();
      const dialog = page.getByRole('dialog', { name: 'Política de privacidad' });
      await expect(dialog).toBeVisible();
      await expect(dialog.getByText(/Versión \d+/)).toBeVisible();
      await expectCoversViewport(page);

      // Escape closes without ticking.
      await page.keyboard.press('Escape');
      await expect(dialog).toBeHidden();
      await expect(
        page.getByRole('checkbox', { name: /Política de privacidad/ }),
      ).not.toBeChecked();

      // "Cerrar" closes without ticking too.
      await link.first().click();
      await expect(dialog).toBeVisible();
      await dialog.getByRole('button', { name: 'Cerrar', exact: true }).last().click();
      await expect(dialog).toBeHidden();
      await expect(
        page.getByRole('checkbox', { name: /Política de privacidad/ }),
      ).not.toBeChecked();

      // Accepting ticks the checkbox.
      await link.first().click();
      await expect(dialog).toBeVisible();
      await page.getByRole('button', { name: 'Aceptar y cerrar' }).click();
      await expect(dialog).toBeHidden();
      await expect(page.getByRole('checkbox', { name: /Política de privacidad/ })).toBeChecked();
    });
  }
});
