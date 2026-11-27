import { appPath } from '../../lib/paths.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { expect, test } from '../../fixtures/test.ts';

const PEOPLE_PATH = '/people';

test.describe('personas (S-27)', () => {
  test('el directorio lista a los miembros y filtra por correo', async ({ as, api }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const listed = (await (await member.get(`organizations/${acme}/members`)).json()) as {
      data: Array<{ name: string; email: string }>;
    };
    const target =
      listed.data.find((item) => item.name && item.email !== 'member@corelog.test') ??
      listed.data[0];
    expect(target, 'the Acme directory must list at least one member').toBeTruthy();

    const page = await as('member');
    await page.goto(appPath(PEOPLE_PATH));
    await expect(page.getByRole('heading', { name: 'Personas' }).first()).toBeVisible();
    await expect(page.getByText(target.name).first()).toBeVisible();

    await page.getByPlaceholder('Buscar por nombre o correo').fill(target.email);
    await expect(page.getByText(target.name).first()).toBeVisible();
  });

  test('la ficha de una persona muestra su nombre', async ({ as, api }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const listed = (await (await member.get(`organizations/${acme}/members`)).json()) as {
      data: Array<{ user_id?: string; id?: string; name: string; email: string }>;
    };
    const target = listed.data.find((item) => item.email === 'lead@corelog.test');
    expect(target, 'lead@corelog.test must be an Acme member').toBeTruthy();
    const userId = target?.user_id ?? target?.id;

    const page = await as('member');
    await page.goto(appPath(`${PEOPLE_PATH}/${userId}`));
    await expect(page.getByText(target?.name ?? '', { exact: false }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Persona no encontrada' })).toHaveCount(0);
  });
});
