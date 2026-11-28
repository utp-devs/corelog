import type { APIRequestContext } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { pickSelectOption } from '../../lib/ui.ts';

const CATALOGS_PATH = '/organization/catalogs';

interface CatalogRow {
  id: string;
  name: string;
  position: number;
  usage_count: number;
}

async function listCatalog(
  client: APIRequestContext,
  orgId: string,
  kind: 'issue-types' | 'priorities' | 'resolutions',
): Promise<CatalogRow[]> {
  const response = await client.get(`organizations/${orgId}/${kind}`);
  expect(response.ok()).toBe(true);
  return ((await response.json()) as { data: CatalogRow[] }).data;
}

async function createItem(
  client: APIRequestContext,
  orgId: string,
  kind: 'issue-types' | 'priorities',
  name: string,
): Promise<CatalogRow> {
  const response = await client.post(`organizations/${orgId}/${kind}`, {
    data: { name, icon: 'star', color: 'purple' },
  });
  expect(response.status()).toBe(201);
  return ((await response.json()) as { data: CatalogRow }).data;
}

/** Best effort cleanup: the item may already be gone because the test deleted it. */
async function removeItem(
  client: APIRequestContext,
  orgId: string,
  kind: 'issue-types' | 'priorities',
  id: string,
  reassignTo?: string,
): Promise<void> {
  const suffix = reassignTo ? `?reassign_to=${reassignTo}` : '';
  await client.delete(`organizations/${orgId}/${kind}/${id}${suffix}`);
}

test.describe('catalogos de la organizacion', () => {
  test('owner crea un tipo de incidencia desde el dialogo y aparece en la lista', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const name = data.uniqueName('tipo');
    const page = await as('owner');
    try {
      await page.goto(appPath(CATALOGS_PATH));
      await expect(
        page.getByRole('heading', { name: 'Tipos, prioridades y resoluciones' }),
      ).toBeVisible();

      await page.getByRole('button', { name: 'Nuevo tipo' }).click();
      const dialog = page.getByRole('dialog');
      await dialog.getByLabel('Nombre').fill(name);
      await dialog.getByRole('radio', { name: 'Rosa' }).click();
      await dialog.getByRole('radio', { name: 'Cohete' }).click();
      await dialog.getByRole('button', { name: 'Guardar' }).click();

      const row = page.locator(`[data-testid="catalog-row"][data-name="${name}"]`);
      await expect(row).toBeVisible();
      await expect(row).toContainText('Sin incidencias');

      const created = (await listCatalog(owner, acme, 'issue-types')).find(
        (item) => item.name === name,
      );
      expect(created).toBeDefined();
    } finally {
      for (const item of await listCatalog(owner, acme, 'issue-types')) {
        if (item.name === name) await removeItem(owner, acme, 'issue-types', item.id);
      }
    }
  });

  test('un nombre repetido se explica en el dialogo en espanol', async ({ as, api, data }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const name = data.uniqueName('rep');
    const first = await createItem(owner, acme, 'priorities', name);
    try {
      const page = await as('owner');
      await page.goto(appPath(CATALOGS_PATH));
      await page.getByRole('tab', { name: 'Prioridades' }).click();
      await page.getByRole('button', { name: 'Nueva prioridad' }).click();
      const dialog = page.getByRole('dialog');
      await dialog.getByLabel('Nombre').fill(name);
      await dialog.getByRole('button', { name: 'Guardar' }).click();
      await expect(dialog.getByRole('alert')).toContainText('Ya existe un elemento con ese nombre');
    } finally {
      await removeItem(owner, acme, 'priorities', first.id);
    }
  });

  test('owner reordena las prioridades con los botones y el orden se guarda', async ({
    as,
    api,
    data,
  }, testInfo) => {
    // Reordering sends the whole priority list of the shared organization, so two projects running
    // it at the same time would invalidate each other's list. The behavior does not depend on the
    // viewport or the theme: run it once.
    test.skip(testInfo.project.name !== 'desktop', 'mutates shared organization data');
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const first = await createItem(owner, acme, 'priorities', data.uniqueName('pa'));
    const second = await createItem(owner, acme, 'priorities', data.uniqueName('pb'));
    try {
      const page = await as('owner');
      await page.goto(appPath(CATALOGS_PATH));
      await page.getByRole('tab', { name: 'Prioridades' }).click();
      await page.getByRole('button', { name: `Subir ${second.name}` }).click();
      await expect(page.locator('.org-catalogs-page__sr')).toHaveText('Orden actualizado');

      const names = (await listCatalog(owner, acme, 'priorities')).map((item) => item.name);
      expect(names.indexOf(second.name)).toBeLessThan(names.indexOf(first.name));

      const rows = page.locator('[data-testid="catalog-row"]');
      const shown = await rows.evaluateAll((items) =>
        items.map((item) => item.getAttribute('data-name')),
      );
      expect(shown.indexOf(second.name)).toBeLessThan(shown.indexOf(first.name));
    } finally {
      await removeItem(owner, acme, 'priorities', first.id);
      await removeItem(owner, acme, 'priorities', second.id);
    }
  });

  test('eliminar un tipo en uso pide a que tipo mover las incidencias y las mueve', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'cat');
    const type = await createItem(owner, acme, 'issue-types', data.uniqueName('uso'));
    const standard = (await listCatalog(owner, acme, 'issue-types')).find(
      (item) => item.name === 'Tarea',
    );
    expect(standard).toBeDefined();
    try {
      const created = await owner.post('issues', {
        data: {
          project_id: project.id,
          summary: 'Incidencia con tipo propio',
          description: 'Prueba de reasignacion',
          issue_type_id: type.id,
        },
      });
      expect(created.status()).toBe(201);
      const issueId = ((await created.json()) as { data: { id: string } }).data.id;

      const page = await as('owner');
      await page.goto(appPath(CATALOGS_PATH));
      const row = page.locator(`[data-testid="catalog-row"][data-name="${type.name}"]`);
      await expect(row).toContainText('1 incidencia');
      await page.getByRole('button', { name: `Eliminar ${type.name}` }).click();

      const dialog = page.getByRole('dialog');
      await expect(dialog.getByTestId('catalog-delete-in-use')).toContainText('1 incidencia');
      await expect(dialog.getByRole('button', { name: 'Eliminar' })).toBeDisabled();
      await pickSelectOption(page, 'Mover las incidencias a', 'Tarea');
      await dialog.getByRole('button', { name: 'Eliminar' }).click();

      await expect(row).toHaveCount(0);
      const moved = await owner.get(`issues/${issueId}`);
      const body = (await moved.json()) as { data: { issue_type: { name: string } } };
      expect(body.data.issue_type.name).toBe('Tarea');
    } finally {
      await removeItem(owner, acme, 'issue-types', type.id, standard?.id);
    }
  });

  test('la epica y la subtarea no se pueden eliminar', async ({ as }) => {
    const page = await as('owner');
    await page.goto(appPath(CATALOGS_PATH));
    await expect(page.getByRole('button', { name: 'Eliminar Épica' })).toBeDisabled();
    await expect(page.getByRole('button', { name: 'Eliminar Subtarea' })).toBeDisabled();
  });

  for (const role of ['member', 'guest'] as const) {
    test(`${role} recibe la pagina 403 en la administracion de catalogos`, async ({ as }) => {
      const page = await as(role);
      await page.goto(appPath(CATALOGS_PATH));
      await expect(
        page.getByRole('heading', { name: 'No tienes acceso a esta página' }),
      ).toBeVisible();
    });
  }

  test('un miembro lee los catalogos pero el API rechaza cambiarlos', async ({ api }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const member = await api('member');
    const read = await member.get(`organizations/${acme}/issue-types`);
    expect(read.status()).toBe(200);
    const write = await member.post(`organizations/${acme}/issue-types`, {
      data: { name: 'No permitido' },
    });
    expect(write.status()).toBe(403);
  });

  test('el lider configura los tipos de incidencia del proyecto', async ({ as, api, data }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'tip');
    const page = await as('owner');

    // Other specs create and delete organization-wide types in parallel: a type listed here can vanish
    // before the save, and the server rejects the stale list. Reload and try again in that case.
    await expect(async () => {
      await page.goto(appPath(`/projects/${project.key}/settings/issue-types`));
      await expect(page.getByRole('checkbox', { name: /Épica/ })).toBeDisabled();
      await expect(page.getByRole('checkbox', { name: /Subtarea/ })).toBeDisabled();
      const bug = page.getByRole('checkbox', { name: /Error/ });
      await expect(bug).toBeChecked();
      await bug.uncheck();
      await page.getByTestId('project-issue-types-save').getByRole('button').click();
      await expect(page.getByText('Tipos de incidencia del proyecto guardados')).toBeVisible({
        timeout: 5_000,
      });
    }).toPass({ timeout: 30_000 });

    const saved = await owner.get(`projects/${project.id}/issue-types`);
    const body = (await saved.json()) as {
      data: { configured: boolean; items: Array<{ name: string }> };
    };
    expect(body.data.configured).toBe(true);
    expect(body.data.items.map((item) => item.name)).not.toContain('Error');
  });
});
