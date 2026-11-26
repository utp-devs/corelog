import { expect, test } from '@playwright/test';

// Smoke test of the three apps served from one origin (see serve:dist or the nginx image).
test('el portal muestra la pantalla de acceso', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole('heading', { name: 'CoreLog' })).toBeVisible();
});

test('el espacio de trabajo exige sesión', async ({ page }) => {
  await page.goto('/workspace/');
  await expect(page).toHaveURL(/\/login\?returnUrl=/);
});
