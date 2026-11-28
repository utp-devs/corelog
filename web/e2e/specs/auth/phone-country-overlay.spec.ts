import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

test.describe('selector de país del teléfono', () => {
  test('el panel se ve completo sobre el formulario en un viewport corto y elige un país', async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1280, height: 560 });
    await page.goto(appPath('/register'));
    await expect(page.getByTestId('register-step-1')).toBeVisible();

    await page.getByRole('button', { name: /^País:/ }).click();
    const search = page.getByRole('combobox', { name: 'Buscar país o prefijo' });
    await expect(search).toBeFocused();
    const panel = page.locator('.phone-input__panel');
    await expect(panel).toBeVisible();

    // The panel lives in the overlay container, fully inside the viewport.
    const box = await panel.boundingBox();
    const viewport = page.viewportSize();
    if (!box || !viewport) throw new Error('panel or viewport missing');
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(viewport.width);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height);

    // Nothing covers it: the element at its centre and near both ends belongs to the panel.
    const covered = await page.evaluate(() => {
      const el = document.querySelector('.phone-input__panel') as HTMLElement;
      const r = el.getBoundingClientRect();
      const points = [
        [r.x + r.width / 2, r.y + r.height / 2],
        [r.x + 8, r.y + 8],
        [r.x + r.width - 8, r.y + r.height - 8],
      ];
      return points.every(([x, y]) => el.contains(document.elementFromPoint(x, y)));
    });
    expect(covered).toBe(true);
    expect(await panel.evaluate((el) => !!el.closest('.cdk-overlay-container'))).toBe(true);

    await search.fill('colom');
    await page.getByRole('option', { name: /Colombia/ }).click();
    await expect(panel).toBeHidden();
    await expect(page.getByRole('button', { name: /^País: Colombia/ })).toBeVisible();
    await expect(page.getByLabel(/^Teléfono/)).toBeFocused();
  });

  test('se cierra con Escape y devuelve el foco al botón', async ({ page }) => {
    await page.goto(appPath('/register'));
    const trigger = page.getByRole('button', { name: /^País:/ });
    await trigger.click();
    await expect(page.locator('.phone-input__panel')).toBeVisible();
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Escape');
    await expect(page.locator('.phone-input__panel')).toBeHidden();
    await expect(trigger).toBeFocused();
  });
});
