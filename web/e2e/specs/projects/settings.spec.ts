import { type APIRequestContext, type Page } from '@playwright/test';

import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { expect, test, type DataHelpers } from '../../fixtures/test.ts';
import type { RoleKey } from '../../fixtures/roles.ts';

interface Project {
  readonly id: string;
  readonly key: string;
  readonly name: string;
}

async function userIdOf(client: APIRequestContext): Promise<string> {
  const response = await client.get('users/me');
  return ((await response.json()) as { data: { id: string } }).data.id;
}

/**
 * A throwaway project led by the owner, with the member as contributor and the viewer as observer.
 * It is created through the API and removed after each test.
 */
async function projectWithTeam(
  api: (role?: RoleKey) => Promise<APIRequestContext>,
  data: DataHelpers,
): Promise<Project> {
  const owner = await api('owner');
  const acme = await orgIdBySlug(owner, 'acme');
  const key = `E${Math.random().toString(36).slice(2, 6).toUpperCase()}`;
  const response = await owner.post('projects', {
    data: {
      organization_id: acme,
      key,
      name: `e2e-settings-${key.toLowerCase()}`,
      type: 'scrum',
      visibility: 'open',
    },
  });
  expect(response.ok(), 'creating the throwaway project').toBeTruthy();
  const project = ((await response.json()) as { data: Project }).data;
  data.trackProject('owner', project);
  for (const [role, projectRole] of [
    ['member', 'contributor'],
    ['viewer', 'viewer'],
  ] as const) {
    const userId = await userIdOf(await api(role));
    const added = await owner.post(`projects/${project.id}/members`, {
      data: { user_id: userId, role: projectRole },
    });
    expect(added.ok(), `adding ${role} to the project`).toBeTruthy();
  }
  return project;
}

function settingsPath(project: Project, section: string): string {
  return appPath(`/projects/${project.key}/settings/${section}`);
}

async function savedToast(page: Page, text: string): Promise<void> {
  await expect(page.getByText(text, { exact: false }).first()).toBeVisible();
}

test.describe('configuración del proyecto', () => {
  test('líder edita, colaborador solo lee y observador no entra', async ({ as, api, data }) => {
    const project = await projectWithTeam(api, data);

    const lead = await as('owner');
    await lead.goto(settingsPath(project, 'details'));
    await expect(lead.getByRole('heading', { name: 'Configuración' })).toBeVisible();
    const name = lead.getByTestId('details-name');
    await expect(name).toBeEnabled();
    await expect(lead.getByRole('button', { name: 'Guardar cambios' })).toBeDisabled();
    await name.fill(`${project.name} v2`);
    await lead.getByRole('button', { name: 'Guardar cambios' }).click();
    await savedToast(lead, 'Detalles del proyecto guardados');

    const member = await as('member');
    await member.goto(settingsPath(project, 'details'));
    await expect(member.getByTestId('settings-readonly')).toContainText('Solo lectura');
    await expect(member.getByTestId('details-name')).toBeDisabled();
    await expect(member.getByRole('button', { name: 'Guardar cambios' })).toHaveCount(0);

    const viewer = await as('viewer');
    await viewer.goto(settingsPath(project, 'details'));
    await expect(viewer.getByText('No tienes acceso a la configuración')).toBeVisible();
  });

  test('el líder agrega una persona y no puede dejar el proyecto sin líder', async ({
    as,
    api,
    data,
  }) => {
    const project = await projectWithTeam(api, data);
    const page = await as('owner');
    await page.goto(settingsPath(project, 'access'));
    await expect(page.getByRole('button', { name: /Quitar a Member User/ })).toBeVisible();

    // The only lead cannot be demoted: the server refuses and the page explains it in Spanish.
    await page.getByRole('combobox', { name: 'Rol de Owner User' }).click();
    await page.getByRole('option', { name: 'Observador', exact: true }).click();
    await expect(page.getByTestId('access-error')).toContainText('al menos un líder');

    await page.getByRole('button', { name: /Quitar a Viewer User/ }).click();
    await page.getByRole('button', { name: 'Quitar', exact: true }).click();
    await savedToast(page, 'salió del proyecto');
    await expect(page.getByRole('button', { name: /Quitar a Viewer User/ })).toHaveCount(0);
  });

  test('matriz de permisos: vista previa, guardar y restaurar el estándar', async ({
    as,
    api,
    data,
  }) => {
    const project = await projectWithTeam(api, data);
    const page = await as('owner');
    await page.goto(settingsPath(project, 'permissions'));
    await expect(page.getByTestId('permissions-default-state')).toContainText('Permisos estándar');

    // The lead's own protected permissions are locked.
    await expect(page.getByTestId('perm-manage_members-lead')).toBeDisabled();

    const box = page.getByTestId('perm-create_issue-viewer');
    await expect(box).not.toBeChecked();
    await box.check();
    await page.getByTestId('permissions-save').click();
    await expect(page.getByTestId('permissions-diff')).toContainText('Crear incidencias');
    await expect(page.getByTestId('permissions-diff')).toContainText('Observador');
    await page.getByTestId('permissions-confirm').click();
    await savedToast(page, 'Permisos guardados');
    await expect(page.getByTestId('permissions-default-state')).toContainText('Personalizados');
    await expect(page.getByTestId('perm-create_issue-viewer')).toBeChecked();

    await page.getByTestId('permissions-reset').click();
    await expect(page.getByTestId('permissions-diff')).toContainText('Crear incidencias');
    await page.getByTestId('permissions-confirm').click();
    await savedToast(page, 'Se restauraron los permisos estándar');
    await expect(page.getByTestId('permissions-default-state')).toContainText('Permisos estándar');
    await expect(page.getByTestId('perm-create_issue-viewer')).not.toBeChecked();
  });

  test('archivar y restaurar con confirmación', async ({ as, api, data }) => {
    const project = await projectWithTeam(api, data);
    const page = await as('owner');
    await page.goto(settingsPath(project, 'danger'));

    await page.getByTestId('danger-archive').getByRole('button').click();
    await expect(page.getByRole('dialog')).toContainText('solo lectura');
    await page.getByRole('dialog').getByRole('button', { name: 'Archivar' }).click();
    await expect(page.getByTestId('settings-archived')).toContainText('Proyecto archivado');

    await page.getByTestId('danger-archive').getByRole('button').click();
    await page.getByRole('dialog').getByRole('button', { name: 'Restaurar' }).click();
    await expect(page.getByTestId('settings-archived')).toHaveCount(0);
    await expect(page.getByTestId('danger-archive')).toContainText('Archivar proyecto');
  });

  test('eliminar pide escribir el nombre y vuelve a la lista de proyectos', async ({
    as,
    api,
    data,
  }) => {
    const project = await projectWithTeam(api, data);
    const page = await as('owner');
    await page.goto(settingsPath(project, 'danger'));

    await page.getByTestId('danger-delete').getByRole('button').click();
    const dialog = page.getByRole('dialog');
    const confirm = dialog.getByRole('button', { name: 'Eliminar proyecto' });
    await expect(confirm).toBeDisabled();
    await dialog.getByRole('textbox').fill(project.name.toUpperCase());
    await expect(confirm).toBeDisabled();
    await dialog.getByRole('textbox').fill(project.name);
    await expect(confirm).toBeEnabled();
    await confirm.click();

    await expect(page).toHaveURL(/\/projects$/);
  });
});
