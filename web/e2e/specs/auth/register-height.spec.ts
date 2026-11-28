import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { fillRegisterStep1, fillRegisterStep2 } from '../../lib/register.ts';

const DATA = {
  firstName: 'Ana',
  lastName: 'Torres',
  phone: '+51 912 345 678',
  email: 'ana.altura@corelog.test',
  password: 'Secreta-12345',
};

/** Height of the rounded container that holds the hero and the form. */
async function containerHeight(page: Page): Promise<number> {
  const box = await page.locator('.auth-layout__shell').boundingBox();
  if (!box) throw new Error('auth container not found');
  return box.height;
}

test.describe('altura del contenedor de acceso', () => {
  test('el registro mide lo mismo que el login en cada paso y con errores', async ({ page }) => {
    await page.goto(appPath('/login'));
    await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
    const login = await containerHeight(page);

    await page.goto(appPath('/register'));
    await expect(page.getByTestId('register-step-1')).toBeVisible();
    const step1 = await containerHeight(page);

    // Errors of the first step.
    await page.getByTestId('register-next').click();
    await expect(page.getByText('Escribe tus nombres')).toBeVisible();
    const step1Errors = await containerHeight(page);

    await fillRegisterStep1(page, DATA);
    await page.getByTestId('register-next').click();
    await expect(page.getByTestId('register-step-2')).toBeVisible();
    const step2 = await containerHeight(page);

    // Errors of the second step, then a long mismatch message.
    await page.getByTestId('register-next').click();
    await expect(page.getByText('Confirma tu contraseña')).toBeVisible();
    const step2Errors = await containerHeight(page);
    await fillRegisterStep2(page, { ...DATA, confirmation: 'otra' });
    await expect(page.getByText('Las contraseñas no coinciden')).toBeVisible();
    const step2Mismatch = await containerHeight(page);

    await page.getByLabel(/^Confirmar contraseña/).fill(DATA.password);
    await page.getByTestId('register-next').click();
    await expect(page.getByTestId('register-step-3')).toBeVisible();
    const step3 = await containerHeight(page);

    for (const height of [step1, step1Errors, step2, step2Errors, step2Mismatch, step3]) {
      expect(Math.abs(height - login)).toBeLessThanOrEqual(1);
    }

    // The form never makes the page scroll.
    const overflow = await page.evaluate(
      () => document.documentElement.scrollHeight - window.innerHeight,
    );
    expect(overflow).toBeLessThanOrEqual(1);
  });
});
