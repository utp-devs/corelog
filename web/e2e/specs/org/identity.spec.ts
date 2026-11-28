import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { loginViaUi } from '../../lib/auth.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { appPath } from '../../lib/paths.ts';

/** The shell of /profile needs an organization: the throwaway account gets one of its own. */
async function giveOrganization(token: string, name: string): Promise<string> {
  const response = await fetch(`${readE2eEnv().apiURL}/organizations`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  if (response.status !== 201)
    throw new Error(`creating the organization returned ${response.status}`);
  return ((await response.json()) as { data: { id: string } }).data.id;
}

async function removeOrganization(token: string, id: string): Promise<void> {
  await fetch(`${readE2eEnv().apiURL}/organizations/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  });
}

async function apiGet<T>(path: string, token: string): Promise<T> {
  const response = await fetch(`${readE2eEnv().apiURL}/${path}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  return ((await response.json()) as { data: T }).data;
}

async function setPhone(token: string, country: string, number: string): Promise<void> {
  const response = await fetch(`${readE2eEnv().apiURL}/users/me`, {
    method: 'PATCH',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ phone: { country, number } }),
  });
  expect(response.status).toBe(200);
}

test.describe('identidad en el perfil', () => {
  test('edita nombres y teléfono y los conserva tras recargar', async ({ browser, data }) => {
    const user = await data.registerUser('identity');
    const orgId = await giveOrganization(user.token, data.uniqueName('identidad'));
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await loginViaUi(page, user.email, user.password);
      await page.goto(appPath('/profile'));
      await expect(page.getByLabel(/^Nombres/)).toHaveValue('Usuario');

      await page.getByLabel(/^Nombres/).fill('María  José');
      await page.getByLabel(/^Apellidos/).fill('Quispe-Huamán');
      await page.getByLabel(/^Teléfono/).fill('+51 987 654 321');
      await expect(page.getByRole('button', { name: /País: Perú/ })).toBeVisible();
      await page.getByRole('button', { name: 'Guardar cambios' }).click();
      await expect(page.getByRole('button', { name: 'Guardar cambios' })).toBeDisabled();

      await page.reload();
      await expect(page.getByLabel(/^Nombres/)).toHaveValue('María José');
      await expect(page.getByLabel(/^Apellidos/)).toHaveValue('Quispe-Huamán');
      await expect(page.getByLabel(/^Teléfono/)).toHaveValue('987 654 321');
      await expect(page.getByText('Sin verificar')).toBeVisible();

      const me = await apiGet<{ first_name: string; phone: { e164: string } }>(
        'users/me',
        user.token,
      );
      expect(me.first_name).toBe('María José');
      expect(me.phone.e164).toBe('+51987654321');
    } finally {
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });

  test('un teléfono inválido o con dígitos en el nombre se explica en el campo', async ({
    browser,
    data,
  }) => {
    const user = await data.registerUser('identity-errors');
    const orgId = await giveOrganization(user.token, data.uniqueName('identidad'));
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await loginViaUi(page, user.email, user.password);
      await page.goto(appPath('/profile'));
      await page.getByLabel(/^Nombres/).fill('Ana2');
      await page.getByRole('button', { name: 'Guardar cambios' }).click();
      await expect(page.getByText('Usa solo letras, espacios, guiones y apóstrofes')).toBeVisible();
      await page.getByLabel(/^Nombres/).fill('Ana');
      await page.getByLabel(/^Teléfono/).fill('+51 12');
      await page.getByRole('button', { name: 'Guardar cambios' }).click();
      await expect(page.getByText('El teléfono no es válido para el país elegido')).toBeVisible();
    } finally {
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });

  test('verificar teléfono: estados del diálogo con el envío y la comprobación interceptados', async ({
    browser,
    data,
  }) => {
    const user = await data.registerUser('verify');
    await setPhone(user.token, 'PE', '987654321');
    const orgId = await giveOrganization(user.token, data.uniqueName('verificar'));
    const context = await browser.newContext();
    const page = await context.newPage();
    // The code is only logged in development, so the endpoints are answered here.
    let sends = 0;
    await page.route('**/users/me/phone/verification', async (route) => {
      sends += 1;
      if (sends === 1) {
        await route.fulfill({
          status: 202,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              destination: '+51******321',
              expires_at: new Date(Date.now() + 600_000).toISOString(),
            },
          }),
        });
      } else {
        await route.fulfill({
          status: 429,
          contentType: 'application/json',
          body: JSON.stringify({ error: { message: 'limit', code: 'user.phone_send_limit' } }),
        });
      }
    });
    let attempts = 0;
    await page.route('**/users/me/phone/verify', async (route) => {
      attempts += 1;
      if (attempts === 1) {
        await route.fulfill({
          status: 422,
          contentType: 'application/json',
          body: JSON.stringify({ error: { message: 'bad', code: 'user.phone_code_invalid' } }),
        });
        return;
      }
      const me = await context.request.get(`${readE2eEnv().apiURL}/users/me`, {
        headers: { Authorization: `Bearer ${user.token}` },
      });
      const body = (await me.json()) as { data: { phone: Record<string, unknown> } };
      body.data.phone = { ...body.data.phone, verified: true };
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(body),
      });
    });
    try {
      await loginViaUi(page, user.email, user.password);
      await page.goto(appPath('/profile'));
      await page.getByRole('button', { name: 'Verificar teléfono' }).click();

      const dialog = page.getByRole('dialog');
      await expect(dialog.getByText('+51******321')).toBeVisible();
      await expect(dialog.getByText(/Podrás pedir otro código en \d+ s/)).toBeVisible();
      await expect(dialog.getByRole('button', { name: 'Verificar', exact: true })).toBeDisabled();

      await dialog.getByLabel('Código de verificación').fill('000000');
      await dialog.getByRole('button', { name: 'Verificar', exact: true }).click();
      await expect(dialog.getByRole('alert')).toContainText('El código no es correcto');

      await dialog.getByLabel('Código de verificación').fill('123456');
      await dialog.getByRole('button', { name: 'Verificar', exact: true }).click();
      await expect(dialog.getByText('Listo, tu teléfono quedó verificado.')).toBeVisible();
      await dialog.getByRole('button', { name: 'Cerrar', exact: true }).last().click();
      await expect(page.getByText('Verificado', { exact: true })).toBeVisible();
    } finally {
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });

  test('cambiar la contraseña exige la confirmación y la valida al instante', async ({
    browser,
    data,
  }) => {
    const user = await data.registerUser('password');
    const orgId = await giveOrganization(user.token, data.uniqueName('clave'));
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await loginViaUi(page, user.email, user.password);
      await page.goto(appPath('/profile/security'));
      const form = page.getByRole('form', { name: 'Cambiar contraseña' });
      await form.getByLabel(/^Contraseña actual/).fill(user.password);
      await form.getByLabel(/^Nueva contraseña/).fill('Nueva-clave-123');
      await form.getByLabel(/^Confirmar nueva contraseña/).fill('Nueva-clave-12');
      await expect(form.getByText('Las contraseñas no coinciden')).toBeVisible();
      await form.getByLabel(/^Nueva contraseña/).fill('Nueva-clave-12');
      await expect(form.getByText('Las contraseñas no coinciden')).toBeHidden();
      await form.getByRole('button', { name: 'Cambiar contraseña' }).click();
      await expect(page.getByText('Contraseña cambiada')).toBeVisible();
    } finally {
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });
});

async function openPrivacy(page: Page): Promise<void> {
  await page.goto(appPath('/profile/privacy'));
  await expect(page.getByTestId('consent-documents')).toBeVisible();
}

test.describe('privacidad y consentimientos', () => {
  test('lista los documentos aceptados, se pueden leer y la autorización de comunicaciones se activa y se retira', async ({
    browser,
    data,
  }) => {
    const user = await data.registerUser('privacy');
    const orgId = await giveOrganization(user.token, data.uniqueName('privacidad'));
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await loginViaUi(page, user.email, user.password);
      await openPrivacy(page);
      const list = page.getByTestId('consent-documents');
      await expect(list.getByText(/Aceptaste la versión \d+ el \d{2}\/\d{2}\/\d{4}/)).toHaveCount(
        3,
      );
      await expect(list.getByText('Obligatorio')).toHaveCount(3);

      await page.getByRole('button', { name: 'Leer Política de privacidad' }).click();
      const dialog = page.getByRole('dialog', { name: 'Política de privacidad' });
      await expect(dialog).toBeVisible();
      await expect(dialog.getByRole('button', { name: 'Aceptar y cerrar' })).toHaveCount(0);
      await page.keyboard.press('Escape');
      await expect(dialog).toBeHidden();

      const toggle = page.getByRole('switch', { name: /comunicaciones y novedades/ });
      await expect(toggle).toHaveAttribute('aria-checked', 'false');
      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-checked', 'true');
      const granted = async () =>
        (
          await apiGet<{ accepted: Array<{ slug: string }> }>('me/consents', user.token)
        ).accepted.map((c) => c.slug);
      await expect.poll(granted).toContain('communications');

      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-checked', 'false');
      await expect.poll(granted).not.toContain('communications');
    } finally {
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });

  test('la pantalla de privacidad cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('member');
    await openPrivacy(page);
    await audit(page, { template: 'content', name: 'profile-privacy' });
  });

  test('el perfil general con teléfono cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('member');
    await page.goto(appPath('/profile'));
    await expect(page.getByLabel(/^Nombres/)).toBeVisible();
    await audit(page, { template: 'content', name: 'profile-general' });
  });
});
