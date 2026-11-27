import { appPath } from '../../lib/paths.ts';
import { acceptInvitationApi, orgIdBySlug } from '../../lib/api.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { pickSelectOption } from '../../lib/ui.ts';
import { expect, test } from '../../fixtures/test.ts';

const MEMBERS_PATH = '/organization/members';

test.describe('miembros de la organizacion', () => {
  test('owner cambia el rol de un miembro tras confirmarlo', async ({ as, api, data }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const invitee = await data.registerUser('role');
    const invite = await data.inviteUser('owner', acme, invitee.email, 'member');
    await acceptInvitationApi(readE2eEnv().apiURL, invite.token, invitee.token);

    try {
      const page = await as('owner');
      await page.goto(appPath(MEMBERS_PATH));
      await pickSelectOption(page, `Rol de ${invitee.name}`, 'Invitado');

      const dialog = page.getByRole('dialog');
      await expect(dialog).toContainText(
        `¿Cambiar el rol de ${invitee.name} de Miembro a Invitado?`,
      );
      await dialog.getByRole('button', { name: 'Cambiar rol' }).click();
      await expect(page.getByText(`Rol de ${invitee.name} actualizado`)).toBeVisible();

      const listed = (await (await owner.get(`organizations/${acme}/members`)).json()) as {
        data: Array<{ user_id?: string; id?: string; role: string }>;
      };
      const member = listed.data.find((item) => (item.user_id ?? item.id) === invitee.id);
      expect(member?.role).toBe('guest');
    } finally {
      await owner.delete(`organizations/${acme}/members/${invitee.id}`);
    }
  });

  test('el API rechaza dejar la organizacion sin propietario', async ({ data }) => {
    const { apiURL } = readE2eEnv();
    const founder = await data.registerUser('lastowner');
    const headers = {
      Authorization: `Bearer ${founder.token}`,
      'Content-Type': 'application/json',
    };
    const created = await fetch(`${apiURL}/organizations`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ name: data.uniqueName('solo') }),
    });
    expect(created.status).toBe(201);
    const org = ((await created.json()) as { data: { id: string } }).data;
    try {
      const demote = await fetch(`${apiURL}/organizations/${org.id}/members/${founder.id}`, {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ role: 'guest' }),
      });
      // Changing one's own role is refused first (org.self_role_change); the last owner rule covers other owners.
      expect([403, 409, 422]).toContain(demote.status);
    } finally {
      await fetch(`${apiURL}/organizations/${org.id}`, { method: 'DELETE', headers });
    }
  });

  test('orgadmin no puede cambiar el rol de un propietario', async ({ as, api }, testInfo) => {
    const acme = await orgIdBySlug(await api('owner'), 'acme');
    const ownerMe = (await (await (await api('owner')).get('users/me')).json()) as {
      data: { id: string; name: string };
    };
    const orgadmin = await api('orgadmin');
    const response = await orgadmin.patch(`organizations/${acme}/members/${ownerMe.data.id}`, {
      data: { role: 'member' },
    });
    expect(response.status()).toBe(403);

    const page = await as('orgadmin');
    await page.goto(appPath(MEMBERS_PATH));
    await expect(page.getByRole('heading', { name: 'Miembros' }).first()).toBeVisible();
    // The table is replaced by cards on narrow viewports.
    test.skip((testInfo.project.use.viewport?.width ?? 0) < 1024, 'desktop table only');
    // An admin gets no role control on an owner's row: the role is shown as text only.
    await expect(page.getByRole('combobox', { name: `Rol de ${ownerMe.data.name}` })).toHaveCount(
      0,
    );
  });

  for (const role of ['member', 'guest'] as const) {
    test(`${role} recibe la pagina 403 en miembros`, async ({ as }) => {
      const page = await as(role);
      await page.goto(appPath(MEMBERS_PATH));
      await expect(
        page.getByRole('heading', { name: 'No tienes acceso a esta página' }),
      ).toBeVisible();
    });
  }
});
