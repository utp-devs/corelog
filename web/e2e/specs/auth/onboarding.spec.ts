import { appPath } from '../../lib/paths.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { loginViaUi } from '../../lib/auth.ts';
import { expect, test } from '../../fixtures/test.ts';

test.describe('onboarding de cuentas nuevas', () => {
  test('la cuenta sin organizacion (newbie) llega a /onboarding', async ({ as }) => {
    const page = await as('newbie');
    await page.goto(appPath('/issues'));
    await expect(page).toHaveURL(/\/onboarding/);
    await expect(page.getByRole('button', { name: 'Crear organización' })).toBeVisible();
  });

  test('registro desde la UI y primer acceso a /onboarding', async ({ browser }) => {
    const { password } = readE2eEnv();
    test.skip(!password, 'E2E_USER_PASSWORD is not set');
    const context = await browser.newContext();
    const page = await context.newPage();
    const email = `e2e.ui.${Date.now().toString(36)}@corelog.test`;
    try {
      await page.goto(appPath('/register'));
      await page.getByLabel(/^Nombres/).fill('Registro');
      await page.getByLabel(/^Apellidos/).fill('Interfaz');
      await page.getByLabel(/^Teléfono/).fill('+51 912 345 678');
      await page.getByLabel('Correo electrónico').fill(email);
      await page.getByLabel(/^Contraseña/).fill(password ?? '');
      await page.getByLabel(/^Confirmar contraseña/).fill(password ?? '');
      for (const name of [
        /Política de privacidad/,
        /tratamiento de mis datos/,
        /Términos de uso/,
      ]) {
        await page.getByRole('checkbox', { name }).check();
      }
      await page.getByRole('button', { name: 'Crear cuenta' }).click();
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      // Accounts have no hard delete: deactivate the account created by this test.
      const token = await page.evaluate(() => window.localStorage.getItem('corelog.auth.token'));
      if (token) {
        await page.request.post(`${readE2eEnv().apiURL}/users/me/deactivate`, {
          headers: { Authorization: `Bearer ${token}` },
          data: { password },
        });
      }
      await context.close();
    }
  });

  test('registro, crear organizacion con slug propio, invitar, copiar enlace y terminar', async ({
    browser,
    data,
  }) => {
    test.setTimeout(180_000);
    const { password } = readE2eEnv();
    test.skip(!password, 'E2E_USER_PASSWORD is not set');
    const user = await data.registerUser('onboard');
    const orgName = data.uniqueName('org');
    const orgSlug = `e2e-${Math.random().toString(36).slice(2, 8)}`;
    const guestEmail = `e2e.guest.${Math.random().toString(36).slice(2, 8)}@corelog.test`;

    const context = await browser.newContext();
    await context.grantPermissions(['clipboard-read', 'clipboard-write'], {
      origin: readE2eEnv().baseURL,
    });
    const page = await context.newPage();
    let createdOrgId: string | undefined;
    try {
      await loginViaUi(page, user.email, password);
      await expect(page).toHaveURL(/\/onboarding/);

      // Step 1: create an organization.
      await page.getByRole('button', { name: 'Crear organización' }).first().click();
      await page.getByLabel('Nombre de la organización').fill(orgName);
      await page.getByLabel('Identificador de la organización').fill(orgSlug);
      await page.locator('form').getByRole('button', { name: 'Crear organización' }).click();

      // Step 2: invite one email and copy its link.
      await page.getByLabel('Correo del invitado 1').fill(guestEmail);
      await page.getByRole('button', { name: 'Crear invitaciones' }).click();
      const copy = page.getByRole('button', { name: `Copiar enlace de ${guestEmail}` });
      await expect(copy).toBeVisible();
      await copy.click();
      await expect(page.getByText('Enlace copiado')).toBeVisible();
      await page.getByRole('button', { name: 'Continuar' }).click();

      // Step 4: finish. The shell shows the new organization in the switcher.
      await page.getByRole('button', { name: 'Ir a proyectos' }).click();
      await expect(page).toHaveURL(/\/projects/);
      await expect(page.getByLabel('Organización actual')).toContainText(orgName);
    } finally {
      // Clean up the organization created through the UI, using the new account's token.
      const token = await page.evaluate(() => window.localStorage.getItem('corelog.auth.token'));
      const api = readE2eEnv().apiURL;
      if (token) {
        const list = await page.request.get(`${api}/organizations`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        const orgs = ((await list.json()) as { data: Array<{ id: string; slug: string }> }).data;
        createdOrgId = orgs.find((org) => org.slug === orgSlug)?.id;
        if (createdOrgId) {
          await page.request.delete(`${api}/organizations/${createdOrgId}`, {
            headers: { Authorization: `Bearer ${token}` },
          });
        }
      }
      await context.close();
    }
    expect(createdOrgId, 'the onboarding must have created the organization').toBeTruthy();
  });
});
