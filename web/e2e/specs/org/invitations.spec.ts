import { appPath } from '../../lib/paths.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { expect, test } from '../../fixtures/test.ts';

const INVITATIONS_PATH = '/organization/invitations';

/** Unique address on the test domain; invitations are revoked or cleaned up, never reused. */
function freshEmail(): string {
  return `e2e.inv.${Math.random().toString(36).slice(2, 10)}@corelog.test`;
}

test.describe('invitaciones de la organizacion (S-22)', () => {
  test('owner crea una invitacion y copia su enlace', async ({ as, api, data }) => {
    const email = freshEmail();
    const page = await as('owner');
    await page
      .context()
      .grantPermissions(['clipboard-read', 'clipboard-write'], { origin: readE2eEnv().baseURL });
    await page.goto(appPath(INVITATIONS_PATH));
    await page.getByLabel('Correos electrónicos').fill(email);
    await page.getByRole('button', { name: 'Enviar invitaciones' }).click();
    await expect(page.getByText('Invitación creada')).toBeVisible();
    await expect(page.getByText(email).first()).toBeVisible();

    await page.getByRole('button', { name: `Copiar enlace de ${email}` }).click();
    await expect(page.getByText('Enlace copiado')).toBeVisible();

    // The invitation is revoked when the test ends, so reruns never collide on the address.
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const listed = (await (await owner.get(`organizations/${acme}/invitations`)).json()) as {
      data: Array<{ id: string; email: string }>;
    };
    const created = listed.data.find((invite) => invite.email === email);
    expect(created, 'the new invitation must be listed').toBeTruthy();
    data.track('owner', `organizations/${acme}/invitations/${created?.id}`);
  });

  test('owner reenvia una invitacion pendiente (nuevo enlace)', async ({
    as,
    api,
    data,
  }, testInfo) => {
    test.skip(
      (testInfo.project.use.viewport?.width ?? 0) < 768,
      'row actions are listed in the desktop table',
    );
    const email = freshEmail();
    const acme = await orgIdBySlug(await api('owner'), 'acme');
    await data.inviteUser('owner', acme, email, 'member');
    const page = await as('owner');
    await page.goto(appPath(INVITATIONS_PATH));
    await expect(page.getByText(email).first()).toBeVisible();
    await page
      .getByRole('row')
      .filter({ hasText: email })
      .getByRole('button', { name: 'Acciones de la fila' })
      .click();
    await page.getByRole('menuitem', { name: 'Reenviar (nuevo enlace)' }).click();
    await expect(
      page.getByText('Nuevo enlace generado. El anterior ya no funciona.'),
    ).toBeVisible();
  });

  test('owner revoca una invitacion pendiente', async ({ as, api, data }, testInfo) => {
    test.skip(
      (testInfo.project.use.viewport?.width ?? 0) < 768,
      'row actions are listed in the desktop table',
    );
    const email = freshEmail();
    const acme = await orgIdBySlug(await api('owner'), 'acme');
    await data.inviteUser('owner', acme, email, 'guest');
    const page = await as('owner');
    await page.goto(appPath(INVITATIONS_PATH));
    await expect(page.getByText(email).first()).toBeVisible();
    await page
      .getByRole('row')
      .filter({ hasText: email })
      .getByRole('button', { name: 'Acciones de la fila' })
      .click();
    await page.getByRole('menuitem', { name: 'Revocar' }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Revocar' }).click();
    await expect(page.getByText('Invitación revocada')).toBeVisible();
  });

  for (const role of ['member', 'guest'] as const) {
    test(`${role} recibe la pagina 403 en invitaciones`, async ({ as }) => {
      const page = await as(role);
      await page.goto(appPath(INVITATIONS_PATH));
      await expect(
        page.getByRole('heading', { name: 'No tienes acceso a esta página' }),
      ).toBeVisible();
    });
  }
});
