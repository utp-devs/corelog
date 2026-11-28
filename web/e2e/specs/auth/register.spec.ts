import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { appPath } from '../../lib/paths.ts';

const REQUIRED = [/Política de privacidad/, /tratamiento de mis datos/, /Términos de uso/];

function uniqueEmail(): string {
  return `reg-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}@corelog.test`;
}

async function fillIdentity(page: Page, email: string, password: string): Promise<void> {
  await page.getByLabel(/^Nombres/).fill('Ana  María');
  await page.getByLabel(/^Apellidos/).fill("Torres O'Brien");
  await page.getByLabel(/^Teléfono/).fill('+51 912 345 678');
  await page.getByLabel('Correo electrónico').fill(email);
  await page.getByLabel(/^Contraseña/).fill(password);
}

/** Accounts have no hard delete: the account the test created is deactivated through the API. */
async function removeAccount(page: Page, password: string): Promise<void> {
  const token = await page.evaluate(() => window.localStorage.getItem('corelog.auth.token'));
  if (!token) return;
  await page.request.post(`${readE2eEnv().apiURL}/users/me/deactivate`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { password },
  });
}

test.describe('registro de cuenta (/register)', () => {
  test('la contraseña y su confirmación deben coincidir y el envío espera los consentimientos', async ({
    page,
  }) => {
    await page.goto(appPath('/register'));
    await expect(page.getByRole('heading', { name: 'Crear cuenta' })).toBeVisible();
    const submit = page.getByRole('button', { name: 'Crear cuenta' });
    await expect(submit).toBeDisabled();

    await fillIdentity(page, uniqueEmail(), 'Secreta-12345');
    await page.getByLabel(/^Confirmar contraseña/).fill('Secreta-1234');
    await expect(page.getByText('Las contraseñas no coinciden')).toBeVisible();

    // Fixing the first field re-evaluates the confirmation; changing it again breaks it again.
    await page.getByLabel(/^Contraseña/).fill('Secreta-1234');
    await expect(page.getByText('Las contraseñas no coinciden')).toBeHidden();
    await page.getByLabel(/^Contraseña/).fill('Secreta-12345');
    await expect(page.getByText('Las contraseñas no coinciden')).toBeVisible();
    await page.getByLabel(/^Confirmar contraseña/).fill('Secreta-12345');
    await expect(page.getByText('Las contraseñas no coinciden')).toBeHidden();

    // Valid data alone is not enough: the three documents must be accepted.
    await expect(submit).toBeDisabled();
    await page.getByRole('checkbox', { name: REQUIRED[0] }).check();
    await page.getByRole('checkbox', { name: REQUIRED[1] }).check();
    await expect(submit).toBeDisabled();
    await page.getByRole('checkbox', { name: REQUIRED[2] }).check();
    await expect(submit).toBeEnabled();
    await expect(page.getByRole('checkbox', { name: /comunicaciones/ })).not.toBeChecked();
  });

  test('los nombres y el teléfono muestran sus reglas en español', async ({ page }) => {
    await page.goto(appPath('/register'));
    await page.getByLabel(/^Nombres/).fill('Ana1');
    await page.getByLabel(/^Apellidos/).fill('  ');
    await page.getByLabel(/^Teléfono/).fill('12');
    await page.getByLabel('Correo electrónico').click();
    await expect(page.getByText('Usa solo letras, espacios, guiones y apóstrofes')).toBeVisible();
    await expect(page.getByText('Escribe tus apellidos')).toBeVisible();
    await expect(page.getByText('El teléfono no es válido para el país elegido')).toBeVisible();
  });

  test('un número internacional pegado cambia el país y se da formato mientras se escribe', async ({
    page,
  }) => {
    await page.goto(appPath('/register'));
    const phone = page.getByLabel(/^Teléfono/);
    await phone.fill('+57 321 123 4567');
    await expect(page.getByRole('button', { name: /País: Colombia/ })).toBeVisible();
    await expect(phone).toHaveValue('321 123 4567');

    await page.getByRole('button', { name: /País:/ }).click();
    await page.getByRole('combobox', { name: /Buscar país/ }).fill('peru');
    await page.getByRole('option', { name: /Perú/ }).click();
    await expect(page.getByRole('button', { name: /País: Perú/ })).toBeVisible();
    await expect(phone).toBeFocused();
  });

  test('cada documento se lee en una ventana y aceptarlo ahí marca su casilla', async ({
    page,
  }) => {
    await page.goto(appPath('/register'));
    const box = page.getByRole('checkbox', { name: REQUIRED[0] });
    await expect(box).not.toBeChecked();

    await page.getByRole('button', { name: 'Política de privacidad', exact: true }).first().click();
    const dialog = page.getByRole('dialog', { name: 'Política de privacidad' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(/Versión \d+/)).toBeVisible();
    await expect(dialog.getByText(/Borrador de plantilla/)).toBeVisible();

    // Escape closes without ticking and returns focus to the link.
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(box).not.toBeChecked();
    await expect(
      page.getByRole('button', { name: 'Política de privacidad', exact: true }).first(),
    ).toBeFocused();

    await page.getByRole('button', { name: 'Política de privacidad', exact: true }).first().click();
    await page.getByRole('button', { name: 'Aceptar y cerrar' }).click();
    await expect(dialog).toBeHidden();
    await expect(box).toBeChecked();
  });

  test('los enlaces del pie abren el documento sin marcar nada', async ({ page }) => {
    await page.goto(appPath('/register'));
    const footer = page.getByRole('navigation', { name: 'Documentos legales' });
    for (const [link, title] of [
      ['Privacidad', 'Política de privacidad'],
      ['Datos personales', 'Política de tratamiento de datos personales'],
      ['Términos', 'Términos de uso'],
      ['Cookies', 'Política de cookies'],
    ] as const) {
      await footer.getByRole('button', { name: link }).click();
      const dialog = page.getByRole('dialog', { name: title });
      await expect(dialog).toBeVisible();
      await expect(dialog.getByRole('button', { name: 'Aceptar y cerrar' })).toHaveCount(0);
      await dialog.getByRole('button', { name: 'Cerrar', exact: true }).last().click();
      await expect(dialog).toBeHidden();
    }
    await expect(page.getByRole('checkbox', { checked: true })).toHaveCount(0);
  });

  test('crea la cuenta con nombres, teléfono y consentimientos y llega a /onboarding', async ({
    browser,
  }) => {
    const { password } = readE2eEnv();
    test.skip(!password, 'E2E_USER_PASSWORD is not set');
    const context = await browser.newContext();
    const page = await context.newPage();
    const email = uniqueEmail();
    try {
      await page.goto(appPath('/register'));
      await fillIdentity(page, email, password ?? '');
      await page.getByLabel(/^Confirmar contraseña/).fill(password ?? '');
      for (const name of REQUIRED) await page.getByRole('checkbox', { name }).check();
      await page.getByRole('checkbox', { name: /comunicaciones/ }).check();
      await page.getByRole('button', { name: 'Crear cuenta' }).click();
      await expect(page).toHaveURL(/\/onboarding/);

      const token = await page.evaluate(() => window.localStorage.getItem('corelog.auth.token'));
      const headers = { Authorization: `Bearer ${token}` };
      const me = await page.request.get(`${readE2eEnv().apiURL}/users/me`, { headers });
      const { data: user } = (await me.json()) as {
        data: {
          first_name: string;
          last_name: string;
          phone: { country: string; number: string; verified: boolean };
          pending_consents: unknown[];
        };
      };
      // Names are trimmed and their inner spaces collapsed before they are sent.
      expect(user.first_name).toBe('Ana María');
      expect(user.last_name).toBe("Torres O'Brien");
      expect(user.phone).toMatchObject({ country: 'PE', number: '912345678', verified: false });
      expect(user.pending_consents).toEqual([]);

      const consents = await page.request.get(`${readE2eEnv().apiURL}/me/consents`, { headers });
      const { data: status } = (await consents.json()) as {
        data: { accepted: Array<{ slug: string }> };
      };
      expect(status.accepted.map((c) => c.slug).sort()).toEqual([
        'communications',
        'data_processing',
        'privacy_policy',
        'terms_of_use',
      ]);
    } finally {
      await removeAccount(page, password ?? '');
      await context.close();
    }
  });

  test('un correo repetido se explica en español junto al campo', async ({ browser, data }) => {
    const { password } = readE2eEnv();
    test.skip(!password, 'E2E_USER_PASSWORD is not set');
    const existing = await data.registerUser('dup');
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await page.goto(appPath('/register'));
      await fillIdentity(page, existing.email, password ?? '');
      await page.getByLabel(/^Confirmar contraseña/).fill(password ?? '');
      for (const name of REQUIRED) await page.getByRole('checkbox', { name }).check();
      await page.getByRole('button', { name: 'Crear cuenta' }).click();
      await expect(page.getByText(/Ya existe una cuenta con este correo/).first()).toBeVisible();
      await expect(page).toHaveURL(/\/register/);
    } finally {
      await context.close();
    }
  });

  test('la pantalla de registro rellena cumple la auditoría de layout', async ({ page, audit }) => {
    await page.goto(appPath('/register'));
    await expect(page.getByRole('heading', { name: 'Crear cuenta' })).toBeVisible();
    await page.getByLabel(/^Nombres/).fill('Ana');
    await page.getByLabel(/^Apellidos/).fill('Torres');
    await page.getByLabel(/^Contraseña/).fill('Secreta-12345');
    await page.getByLabel(/^Confirmar contraseña/).fill('otra');
    await expect(page.getByText('Las contraseñas no coinciden')).toBeVisible();
    await audit(page, { template: 'split', name: 'register-errors' });
  });
});
