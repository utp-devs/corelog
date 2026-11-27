import { appPath } from '../../lib/paths.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { expect, test } from '../../fixtures/test.ts';

const TEAMS_PATH = '/teams';

test.describe('equipos (S-25)', () => {
  test('member crea un equipo desde la pantalla de equipos', async ({ as, api }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const name = `e2e-team-${Math.random().toString(36).slice(2, 8)}`;

    const page = await as('member');
    await page.goto(appPath(TEAMS_PATH));
    await page.getByRole('button', { name: 'Crear equipo' }).first().click();
    await page.getByLabel('Nombre del equipo').fill(name);
    await page.getByRole('dialog').getByRole('button', { name: 'Guardar' }).click();
    await expect(page.getByText(name, { exact: true }).first()).toBeVisible();

    // Clean up through the API: the creator may delete the team.
    const listed = (await (
      await member.get('teams', { params: { organization_id: acme } })
    ).json()) as {
      data: Array<{ id: string; name: string }>;
    };
    const created = listed.data.find((team) => team.name === name);
    expect(created, 'the team must exist in the API').toBeTruthy();
    if (created) await member.delete(`teams/${created.id}`);
  });

  test('member agrega a una persona al equipo', async ({ as, api, data }) => {
    const acme = await orgIdBySlug(await api('member'), 'acme');
    const team = await data.createTeam('member', acme, 'team');
    const page = await as('member');
    await page.goto(appPath(`/teams/${team.id}`));
    await page
      .getByRole('group', { name: 'Agregar miembro' })
      .getByRole('combobox')
      .first()
      .fill('Viewer');
    await page
      .getByRole('option', { name: /Viewer User/ })
      .first()
      .click();
    await page
      .getByRole('group', { name: 'Agregar miembro' })
      .getByRole('button', { name: 'Agregar miembro' })
      .click();
    await expect(
      page.getByRole('button', { name: /Quitar a Viewer User del equipo/ }),
    ).toBeVisible();
  });

  test('la pagina de equipo muestra la seccion de miembros', async ({ as, api, data }) => {
    const acme = await orgIdBySlug(await api('member'), 'acme');
    const team = await data.createTeam('member', acme, 'tabs');
    const page = await as('member');
    await page.goto(appPath(`/teams/${team.id}`));
    await expect(page.getByRole('heading', { name: 'Miembros' })).toBeVisible();
  });

  test('guest ve los equipos en solo lectura', async ({ as }) => {
    const page = await as('guest');
    await page.goto(appPath(TEAMS_PATH));
    await expect(page.getByRole('heading', { name: 'Equipos' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Crear equipo' })).toHaveCount(0);
  });

  // Team creation follows the organization policy (members by default), not the project role:
  // a viewer is an organization member and may create teams.
  test('viewer, miembro de la organizacion, puede crear equipos segun la politica', async ({
    as,
  }) => {
    const page = await as('viewer');
    await page.goto(appPath(TEAMS_PATH));
    await expect(page.getByRole('heading', { name: 'Equipos' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Crear equipo' }).first()).toBeVisible();
  });

  test('guest no puede crear equipos por API', async ({ api }) => {
    const guest = await api('guest');
    const acme = await orgIdBySlug(guest, 'acme');
    const response = await guest.post('teams', {
      data: { organization_id: acme, name: 'e2e-guest-nope' },
    });
    expect(response.status()).toBe(403);
  });
});
