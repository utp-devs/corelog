import { appPath } from '../../lib/paths.ts';
import { loginViaUi } from '../../lib/auth.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { expect, test } from '../../fixtures/test.ts';

test.describe('aceptar invitaciones (/invite/:token)', () => {
  test('token inexistente muestra el estado no valido', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/invite/token-que-no-existe-e2e'));
    await expect(page.getByText('Este enlace no es válido.')).toBeVisible();
  });

  test('invitacion revocada muestra el estado no disponible', async ({ browser, api, data }) => {
    const invitee = await data.registerUser('revoked');
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const invite = await data.inviteUser('owner', acme, invitee.email, 'member');
    const revoke = await owner.delete(`organizations/${acme}/invitations/${invite.id}`);
    expect(revoke.ok(), 'revoking the invitation should succeed').toBe(true);

    const context = await browser.newContext();
    try {
      const page = await context.newPage();
      await page.goto(appPath(`/invite/${invite.token}`));
      await expect(page.getByText(/Esta invitación fue revocada/)).toBeVisible();
    } finally {
      await context.close();
    }
  });

  test.skip('invitacion caducada muestra el estado caducado', () => {
    // No API backdates expires_at, so the expired state is covered by backend tests only.
  });

  test('email distinto al de la invitacion avisa del cambio de cuenta', async ({
    as,
    api,
    data,
  }) => {
    const invitee = await data.registerUser('mismatch');
    const acme = await orgIdBySlug(await api('owner'), 'acme');
    const invite = await data.inviteUser('owner', acme, invitee.email, 'guest');

    const page = await as('member');
    await page.goto(appPath(`/invite/${invite.token}`));
    await expect(page.getByText(`Esta invitación es para ${invitee.email}`)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Cambiar de cuenta' })).toBeVisible();
  });

  test('invitacion aceptada muestra el estado usado', async ({ browser, api, data }) => {
    const invitee = await data.registerUser('used');
    const acme = await orgIdBySlug(await api('owner'), 'acme');
    const invite = await data.inviteUser('owner', acme, invitee.email, 'member');
    const accepted = await fetch(`${readE2eEnv().apiURL}/invitations/${invite.token}/accept`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${invitee.token}` },
    });
    expect(accepted.status).toBe(201);

    const context = await browser.newContext();
    try {
      const page = await context.newPage();
      await page.goto(appPath(`/invite/${invite.token}`));
      await expect(page.getByText(/Esta invitación ya se ha utilizado/)).toBeVisible();
    } finally {
      await context.close();
    }
  });

  test('invitado nuevo inicia sesion, acepta y ve la organizacion', async ({
    browser,
    api,
    data,
  }) => {
    const invitee = await data.registerUser('accept');
    const acme = await orgIdBySlug(await api('owner'), 'acme');
    const invite = await data.inviteUser('owner', acme, invitee.email, 'member');

    const context = await browser.newContext();
    try {
      const page = await context.newPage();
      await page.goto(appPath(`/invite/${invite.token}`));
      await expect(page.locator('body')).toContainText(invitee.email);
      await page.getByRole('button', { name: 'Iniciar sesión' }).click();
      await loginViaUi(page, invitee.email, invitee.password);
      // An account without an organization is sent to onboarding; wait for that redirect before navigating again.
      await expect(page).toHaveURL(/\/onboarding$/);
      await page.waitForLoadState('load');

      await page.goto(appPath(`/invite/${invite.token}`));
      await page.getByRole('button', { name: 'Aceptar invitación' }).click();
      await expect(page).not.toHaveURL(new RegExp(`/invite/${invite.token}`));

      await page.goto(appPath('/issues'));
      await expect(page.getByLabel('Organización actual')).toContainText('Acme');
    } finally {
      await context.close();
    }
  });
});
