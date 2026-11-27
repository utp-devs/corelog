import type { APIRequestContext, APIResponse } from '@playwright/test';

import { orgIdBySlug } from '../lib/api.ts';
import { appPath } from '../lib/paths.ts';
import { findProjectByKey, randomProjectKey, type ApiProject } from '../lib/projects-api.ts';
import { expect, test, type DataHelpers } from '../fixtures/test.ts';
import type { RoleKey } from '../fixtures/roles.ts';

// Life of a project, end to end: create it through the wizard, tune its permissions, keep it private,
// archive and restore it, and delete it. API fixtures prepare and verify; the UI does what a person does.
// Desktop only: the layout of each screen is covered by the other journeys.

type ApiFor = (role?: RoleKey) => Promise<APIRequestContext>;

async function userIdOf(client: APIRequestContext): Promise<string> {
  const response = await client.get('users/me');
  return ((await response.json()) as { data: { id: string } }).data.id;
}

/** Creates a project as the owner (its lead) and registers it for cleanup. */
async function ownerProject(
  api: ApiFor,
  data: DataHelpers,
  visibility: 'open' | 'private' = 'open',
): Promise<ApiProject> {
  const owner = await api('owner');
  const acme = await orgIdBySlug(owner, 'acme');
  const key = randomProjectKey();
  const response = await owner.post('projects', {
    data: {
      organization_id: acme,
      key,
      name: data.uniqueName('life'),
      type: 'kanban',
      template: 'kanban',
      visibility,
    },
  });
  expect(response.status(), 'creating the project').toBe(201);
  const project = ((await response.json()) as { data: ApiProject }).data;
  data.trackProject('owner', project);
  return project;
}

async function addMember(
  api: ApiFor,
  project: ApiProject,
  role: RoleKey,
  projectRole: 'contributor' | 'viewer',
): Promise<void> {
  const owner = await api('owner');
  const added = await owner.post(`projects/${project.id}/members`, {
    data: { user_id: await userIdOf(await api(role)), role: projectRole },
  });
  expect(added.ok(), `adding ${role} as ${projectRole}`).toBeTruthy();
}

function createIssue(client: APIRequestContext, project: ApiProject): Promise<APIResponse> {
  return client.post('issues', {
    data: {
      summary: 'Incidencia del ciclo de vida',
      description: 'Creada por el recorrido del proyecto.',
      priority: 'medium',
      project_id: project.id,
    },
  });
}

async function errorCodeOf(response: APIResponse): Promise<string | undefined> {
  const body = (await response.json()) as { error?: { code?: string }; code?: string };
  return body.error?.code ?? body.code;
}

/** A project write that the archive rules cover: the details of the project, edited by its lead. */
async function editProject(client: APIRequestContext, project: ApiProject): Promise<APIResponse> {
  return client.patch(`projects/${project.id}`, {
    data: {
      name: project.name,
      description: `Editado ${Math.random().toString(36).slice(2, 8)}`,
      type: 'kanban',
      lead_id: await userIdOf(client),
      visibility: 'open',
    },
  });
}

function settingsPath(project: ApiProject, section: string): string {
  return appPath(`/projects/${project.key}/settings/${section}`);
}

test.describe('ciclo de vida de un proyecto', () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(
      testInfo.project.name !== 'desktop',
      'El recorrido se ejecuta en el proyecto desktop',
    );
  });

  test('un member crea un proyecto Kanban con el asistente y aterriza en él', async ({
    as,
    api,
    data,
  }) => {
    const member = await api('member');
    const acme = await orgIdBySlug(member, 'acme');
    const name = data.uniqueName('kan');
    const key = randomProjectKey();

    const page = await as('member');
    try {
      await page.goto(appPath('/projects/new'));
      await page.getByTestId('template-kanban').click();
      await expect(page.getByTestId('template-kanban').locator('input')).toBeChecked();
      await page.getByTestId('wizard-next').click();
      await page.getByLabel('Nombre').fill(name);
      await page.getByLabel('Clave').fill(key);
      await expect(page.getByText('Clave disponible.')).toBeVisible();
      await page.getByTestId('wizard-next').click();
      await page.getByTestId('wizard-next').click();
      await expect(page.getByTestId('wizard-summary')).toContainText('Kanban');
      await page.getByTestId('wizard-create').click();

      await expect(page).toHaveURL(new RegExp(`/projects/${key}(/|$)`));
      const created = await findProjectByKey(member, acme, key);
      expect(created, 'the project exists in the API').not.toBeNull();
    } finally {
      const created = await findProjectByKey(member, acme, key);
      if (created) data.trackProject('member', created);
    }
  });

  test('el líder abre un permiso al observador, se lo quita al colaborador y restaura el estándar', async ({
    as,
    api,
    data,
  }) => {
    const project = await ownerProject(api, data);
    await addMember(api, project, 'member', 'contributor');
    await addMember(api, project, 'viewer', 'viewer');
    const viewer = await api('viewer');
    const contributor = await api('member');

    // Standard permissions: the observer cannot create issues, the collaborator can.
    expect((await createIssue(viewer, project)).status()).toBe(403);

    const lead = await as('owner');
    await lead.goto(settingsPath(project, 'permissions'));
    await expect(lead.getByTestId('permissions-default-state')).toContainText('Permisos estándar');

    await lead.getByTestId('perm-create_issue-viewer').check();
    await lead.getByTestId('permissions-save').click();
    await lead.getByTestId('permissions-confirm').click();
    await expect(lead.getByTestId('permissions-default-state')).toContainText('Personalizados');
    expect((await createIssue(viewer, project)).status()).toBe(201);

    await lead.getByTestId('perm-create_issue-contributor').uncheck();
    await lead.getByTestId('permissions-save').click();
    await lead.getByTestId('permissions-confirm').click();
    await expect(lead.getByTestId('perm-create_issue-contributor')).not.toBeChecked();
    expect((await createIssue(contributor, project)).status()).toBe(403);

    await lead.getByTestId('permissions-reset').click();
    await lead.getByTestId('permissions-confirm').click();
    await expect(lead.getByTestId('permissions-default-state')).toContainText('Permisos estándar');
    expect((await createIssue(contributor, project)).status()).toBe(201);
    expect((await createIssue(viewer, project)).status()).toBe(403);
  });

  test('un proyecto privado no existe para quien no es miembro', async ({ as, api, data }) => {
    const project = await ownerProject(api, data, 'private');
    const outsider = await api('member');
    const acme = await orgIdBySlug(outsider, 'acme');

    const direct = await outsider.get(`projects/${project.key}`, {
      params: { organization_id: acme },
    });
    expect([403, 404]).toContain(direct.status());

    const page = await as('member');
    await page.goto(appPath(`/projects/${project.key}/board`));
    await expect(page.getByText('No encontramos esta página')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Tablero' })).toHaveCount(0);

    await page.goto(appPath('/projects'));
    await page.getByRole('textbox', { name: 'Buscar proyectos' }).fill(project.name);
    await expect(page.getByText('Ningún proyecto coincide')).toBeVisible();
    await expect(page.getByTestId(`project-row-${project.key}`)).toHaveCount(0);

    // The lead still sees it.
    const lead = await as('owner');
    await lead.goto(appPath('/projects'));
    await lead.getByRole('textbox', { name: 'Buscar proyectos' }).fill(project.name);
    await expect(lead.getByTestId(`project-row-${project.key}`)).toBeVisible();
  });

  test('archivar deja el proyecto en solo lectura y restaurar devuelve la escritura', async ({
    as,
    api,
    data,
  }) => {
    const project = await ownerProject(api, data);
    const owner = await api('owner');
    const created = await createIssue(owner, project);
    expect(created.status()).toBe(201);
    const issueId = ((await created.json()) as { data: { id: string } }).data.id;

    expect((await editProject(owner, project)).status()).toBe(200);

    const page = await as('owner');
    await page.goto(settingsPath(project, 'danger'));
    await page.getByTestId('danger-archive').getByRole('button').click();
    await page.getByRole('dialog').getByRole('button', { name: 'Archivar' }).click();
    await expect(page.getByTestId('settings-archived')).toContainText('Proyecto archivado');
    await expect(page.getByTestId('sidebar-project-archived')).toBeVisible();

    // The server refuses the writes of an archived project with a conflict.
    const refused = await editProject(owner, project);
    expect(refused.status()).toBe(409);
    expect(await errorCodeOf(refused)).toBe('project.archived');

    // The board and the issue explain it in Spanish and offer no write controls.
    await page.goto(appPath(`/projects/${project.key}/board`));
    await expect(page.getByTestId('archived-notice')).toContainText(
      'Este proyecto está archivado y es de solo lectura',
    );
    await expect(page.getByText('project.archived')).toHaveCount(0);
    await expect(page.getByRole('button', { name: /^Mover .* a columna$/ })).toHaveCount(0);

    await page.goto(appPath(`/browse/${issueId}`));
    await expect(page.getByTestId('archived-notice')).toBeVisible();
    await expect(page.getByRole('button', { name: /^Editar Resumen/ })).toBeDisabled();
    await expect(page.getByText('project.archived')).toHaveCount(0);

    // A manager restores it from the notice and writes work again.
    await page.getByTestId('archived-notice-restore').getByRole('button').click();
    await expect(page.getByTestId('archived-notice')).toHaveCount(0);
    expect((await editProject(owner, project)).status()).toBe(200);
  });

  test('eliminar con el nombre escrito vuelve a la lista y el proyecto desaparece', async ({
    as,
    api,
    data,
  }) => {
    const project = await ownerProject(api, data);
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');

    const page = await as('owner');
    await page.goto(settingsPath(project, 'danger'));
    await page.getByTestId('danger-delete').getByRole('button').click();
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('textbox').fill(project.name);
    await dialog.getByRole('button', { name: 'Eliminar proyecto' }).click();

    await expect(page).toHaveURL(/\/projects$/);
    expect(await findProjectByKey(owner, acme, project.key)).toBeNull();
  });
});
