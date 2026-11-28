import { expect, test } from '../../fixtures/test.ts';
import type { RoleKey } from '../../fixtures/roles.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';

// Dashboards of gadgets: the home dashboard, the manager, the editor and sharing. The API prepares data and
// verifies; the UI is used for what the person does. Created dashboards are deleted by the data fixture.

interface ApiDashboard {
  readonly id: string;
  readonly name: string;
}

test.describe('paneles', () => {
  test('el panel principal muestra sus gadgets con el banner de bienvenida', async ({
    as,
    audit,
  }) => {
    const page = await as('member');
    await page.goto(appPath('/dashboard'));
    await expect(page.locator('[data-page="dashboard"][data-ready="true"]')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Mi resumen', level: 1 })).toBeVisible();
    await expect(page.locator('.dashboard-welcome__greeting')).toContainText('Hola');
    await expect(page.getByTestId('gadget-assigned_to_me')).toBeVisible();
    await expect(page.getByTestId('gadget-activity_stream')).toBeVisible();
    await expect(page.getByTestId('gadget-created_vs_resolved')).toBeVisible();
    await audit(page, { template: 'wide', name: 'dashboard-home' });
  });

  test('el gestor de paneles pasa la auditoria de maquetacion', async ({ as, audit }) => {
    const page = await as('member');
    await page.goto(appPath('/dashboards'));
    await expect(page.locator('[data-page="dashboards"][data-ready="true"]')).toBeVisible();
    await expect(page.getByTestId('dashboards-list')).toBeVisible();
    await audit(page, { template: 'wide', name: 'dashboards-manager' });
  });

  test.describe('flujos', () => {
    test.beforeEach(({}, testInfo) => {
      test.skip(
        testInfo.project.name !== 'desktop',
        'Los flujos se ejecutan en el proyecto desktop',
      );
    });

    test('crear un panel desde el gestor, anadir un gadget, guardar y recargar', async ({
      as,
      data,
    }) => {
      const page = await as('member');
      const name = data.uniqueName('panel');
      await page.goto(appPath('/dashboards'));
      await page.getByTestId('dashboards-create').click();
      await page.getByTestId('dashboard-name').fill(name);
      await page.getByTestId('dashboard-form-confirm').getByRole('button').click();

      await expect(page).toHaveURL(/\/dashboards\/[0-9a-f-]{36}$/);
      const id = page.url().split('/').pop() as string;
      data.track('member', `dashboards/${id}`);
      await expect(page.getByRole('heading', { name, level: 1 })).toBeVisible();
      await expect(page.getByText('Este panel aún no tiene gadgets')).toBeVisible();

      await page.getByTestId('dashboard-edit').click();
      await page.getByTestId('dashboard-add-gadget').click();
      await page.getByTestId('catalog-text').click();
      await page.getByTestId('gadget-title').fill('Aviso del equipo');
      await page.getByTestId('gadget-field-markdown').fill('Reunión **el lunes** a las 10.');
      await page.getByTestId('gadget-config-apply').getByRole('button').click();
      await expect(page.getByTestId('gadget-text')).toContainText('Reunión');

      await page.getByTestId('dashboard-save').click();
      await expect(page.getByTestId('dashboard-edit')).toBeVisible();

      await page.reload();
      await expect(page.getByTestId('gadget-text')).toContainText('el lunes');
      await expect(page.getByRole('heading', { name: 'Aviso del equipo' })).toBeVisible();
    });

    test('mover un gadget a otra columna con el menu y guardar el orden', async ({
      as,
      api,
      data,
    }) => {
      const member = await api('member');
      const created = await data.create<ApiDashboard>('member', 'dashboards', {
        name: data.uniqueName('columnas'),
      });
      data.track('member', `dashboards/${created.id}`);
      const saved = await member.put(`dashboards/${created.id}/gadgets`, {
        data: {
          layout: '2',
          gadgets: [
            {
              type: 'text',
              title: 'Uno',
              config: { markdown: 'uno' },
              column_index: 0,
              position: 0,
            },
            {
              type: 'text',
              title: 'Dos',
              config: { markdown: 'dos' },
              column_index: 0,
              position: 1,
            },
          ],
        },
      });
      expect(saved.ok()).toBe(true);

      const page = await as('member');
      await page.goto(appPath(`/dashboards/${created.id}`));
      await page.getByTestId('dashboard-edit').click();
      await page.getByRole('button', { name: 'Opciones de Uno' }).click();
      await page.getByRole('menuitem', { name: 'Mover a la columna 2' }).click();
      await page.getByTestId('dashboard-save').click();
      await expect(page.getByTestId('dashboard-edit')).toBeVisible();

      const stored = (await (await member.get(`dashboards/${created.id}`)).json()) as {
        data: { gadgets: Array<{ title: string; column_index: number }> };
      };
      expect(stored.data.gadgets.find((g) => g.title === 'Uno')?.column_index).toBe(1);
      expect(stored.data.gadgets.find((g) => g.title === 'Dos')?.column_index).toBe(0);
    });

    test('un panel compartido se ve en solo lectura para otro rol', async ({ as, api, data }) => {
      const member = await api('member');
      const acme = await orgIdBySlug(member, 'acme');
      const created = await data.create<ApiDashboard>('member', 'dashboards', {
        name: data.uniqueName('compartido'),
      });
      data.track('member', `dashboards/${created.id}`);
      await member.put(`dashboards/${created.id}/gadgets`, {
        data: {
          gadgets: [
            { type: 'text', title: 'Nota', config: { markdown: 'Para todos' }, column_index: 0 },
          ],
        },
      });
      const shared = await member.put(`dashboards/${created.id}/shares`, {
        data: { shares: [{ type: 'organization', target_id: acme, permission: 'view' }] },
      });
      expect(shared.ok()).toBe(true);

      const page = await as('viewer');
      await page.goto(appPath('/dashboards'));
      await expect(page.getByRole('link', { name: created.name })).toBeVisible();
      await page.getByRole('link', { name: created.name }).click();

      await expect(page.getByTestId('gadget-text')).toContainText('Para todos');
      await expect(page.getByTestId('dashboard-readonly')).toBeVisible();
      await expect(page.getByTestId('dashboard-edit')).toHaveCount(0);
      await expect(page.getByTestId('dashboard-share')).toHaveCount(0);

      const viewer = await api('viewer');
      const attempt = await viewer.put(`dashboards/${created.id}/gadgets`, {
        data: { gadgets: [] },
      });
      expect(attempt.status()).toBe(403);
    });

    test('un panel compartido nunca muestra incidencias de un proyecto privado a quien no es miembro', async ({
      as,
      api,
      data,
    }) => {
      const owner = await api('owner');
      const acme = await orgIdBySlug(owner, 'acme');
      const project = await data.create<{ id: string; key: string; name: string }>(
        'owner',
        'projects',
        {
          organization_id: acme,
          key: `P${Math.random().toString(36).slice(2, 6).toUpperCase()}`,
          name: data.uniqueName('privado'),
          type: 'kanban',
          visibility: 'private',
        },
      );
      data.trackProject('owner', project);
      const secret = `Secreto ${Math.random().toString(36).slice(2, 8)}`;
      await createIssueViaApi(owner, project.id, secret);

      const dashboard = await data.create<ApiDashboard>('owner', 'dashboards', {
        name: data.uniqueName('fugas'),
      });
      data.track('owner', `dashboards/${dashboard.id}`);
      const put = await owner.put(`dashboards/${dashboard.id}/gadgets`, {
        data: {
          gadgets: [
            {
              type: 'filter_results',
              title: 'Del proyecto privado',
              config: { query: `project = ${project.key}`, columns: ['key', 'summary'] },
              column_index: 0,
            },
          ],
        },
      });
      expect(put.ok(), await put.text()).toBe(true);
      await owner.put(`dashboards/${dashboard.id}/shares`, {
        data: { shares: [{ type: 'organization', target_id: acme, permission: 'view' }] },
      });

      const ownerPage = await as('owner');
      await ownerPage.goto(appPath(`/dashboards/${dashboard.id}`));
      await expect(ownerPage.getByTestId('gadget-filter_results')).toContainText(secret);

      const viewerPage = await as('viewer');
      await viewerPage.goto(appPath(`/dashboards/${dashboard.id}`));
      await expect(viewerPage.getByTestId('gadget-filter_results')).toBeVisible();
      await expect(viewerPage.getByTestId('gadget-filter_results')).toContainText(
        'El filtro no devuelve incidencias',
      );
      await expect(viewerPage.locator('body')).not.toContainText(secret);
    });

    test('duplicar un panel visible crea una copia propia y editable', async ({
      as,
      api,
      data,
    }) => {
      const owner = await api('owner');
      const acme = await orgIdBySlug(owner, 'acme');
      const original = await data.create<ApiDashboard>('owner', 'dashboards', {
        name: data.uniqueName('original'),
      });
      data.track('owner', `dashboards/${original.id}`);
      await owner.put(`dashboards/${original.id}/shares`, {
        data: { shares: [{ type: 'organization', target_id: acme, permission: 'view' }] },
      });

      const page = await as('member');
      await page.goto(appPath(`/dashboards/${original.id}`));
      await page.getByTestId('dashboard-more').click();
      await page.getByRole('menuitem', { name: 'Duplicar' }).click();
      await expect(page).not.toHaveURL(new RegExp(original.id));
      await expect(page).toHaveURL(/\/dashboards\/[0-9a-f-]{36}$/);
      const copyId = page.url().split('/').pop() as string;
      expect(copyId).not.toBe(original.id);
      data.track('member', `dashboards/${copyId}`);
      await expect(
        page.getByRole('heading', { name: `Copia de ${original.name}`, level: 1 }),
      ).toBeVisible();
      await expect(page.getByTestId('dashboard-edit')).toBeVisible();
    });

    test('un panel propio se puede eliminar escribiendo su nombre', async ({ as, data }) => {
      const role: RoleKey = 'member';
      const created = await data.create<ApiDashboard>(role, 'dashboards', {
        name: data.uniqueName('borrar'),
      });
      data.track(role, `dashboards/${created.id}`);

      const page = await as(role);
      await page.goto(appPath('/dashboards'));
      await page.getByTestId(`dashboard-actions-${created.id}`).click();
      await page.getByRole('menuitem', { name: 'Eliminar' }).click();
      const dialog = page.getByRole('dialog');
      const confirm = dialog.getByRole('button', { name: 'Eliminar panel' });
      await expect(confirm).toBeDisabled();
      await dialog.getByRole('textbox').fill(created.name);
      await confirm.click();
      await expect(page.getByTestId(`dashboard-row-${created.id}`)).toHaveCount(0);
    });
  });
});
