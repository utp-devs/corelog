import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { projectWithIssue } from './issue-fixtures.ts';

test.describe('crear incidencia', () => {
  test('el diálogo crea una incidencia, ofrece crear otra y abrirla', async ({ as, api, data }) => {
    const { project } = await projectWithIssue(api, data, 'crt');
    const page = await as('member');
    await page.goto(appPath(`/projects/${project.key}/board`));

    await page.getByRole('button', { name: 'Crear', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: 'Crear incidencia' });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('textbox', { name: /Resumen/ }).fill('Creada desde el diálogo');
    await dialog.getByRole('textbox', { name: 'Descripción' }).click();
    await page.keyboard.type('Descripción de la incidencia creada en la prueba.');
    await dialog.getByRole('button', { name: 'Crear incidencia' }).click();

    await expect(page.getByRole('dialog', { name: 'Incidencia creada' })).toBeVisible();
    await page.getByRole('button', { name: 'Crear otra' }).click();
    await expect(page.getByRole('dialog', { name: 'Crear incidencia' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: /Resumen/ })).toHaveValue('');
  });

  test('el atajo c abre el diálogo y un campo obligatorio del proyecto bloquea el envío', async ({
    as,
    api,
    data,
  }) => {
    const { owner, project } = await projectWithIssue(api, data, 'crtreq');
    const acme = (await (await owner.get('organizations')).json()) as {
      data: { id: string; slug: string }[];
    };
    const orgId = acme.data.find((org) => org.slug === 'acme')?.id ?? '';
    const created = await owner.post(`organizations/${orgId}/custom-fields`, {
      data: { name: data.uniqueName('cf').slice(0, 30), type: 'text' },
    });
    expect(created.ok(), 'creating the custom field').toBe(true);
    const field = ((await created.json()) as { data: { id: string; name: string } }).data;
    data.track('owner', `organizations/${orgId}/custom-fields/${field.id}`);
    const contexts = await owner.put(`organizations/${orgId}/custom-fields/${field.id}/contexts`, {
      data: { contexts: [{ project_id: project.id, issue_type_id: null, required: true }] },
    });
    expect(contexts.ok(), 'adding the context').toBe(true);

    const page = await as('member');
    await page.goto(appPath(`/projects/${project.key}/board`));
    await page.waitForLoadState('networkidle');
    await page.keyboard.press('c');
    const dialog = page.getByRole('dialog', { name: 'Crear incidencia' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText(field.name)).toBeVisible();
    await dialog.getByRole('textbox', { name: /Resumen/ }).fill('Con campo obligatorio');
    await dialog.getByRole('textbox', { name: 'Descripción' }).click();
    await page.keyboard.type('Descripción suficiente para la prueba.');
    await dialog.getByRole('button', { name: 'Crear incidencia' }).click();
    await expect(dialog.getByRole('alert')).toContainText(`Completa el campo «${field.name}»`);

    await dialog.getByRole('textbox', { name: field.name }).fill('Valor');
    await dialog.getByRole('textbox', { name: field.name }).blur();
    await dialog.getByRole('button', { name: 'Crear incidencia' }).click();
    await expect(page.getByRole('dialog', { name: 'Incidencia creada' })).toBeVisible();
  });
});
