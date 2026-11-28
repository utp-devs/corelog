import type { Page } from '@playwright/test';

import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { expect, test } from '../../fixtures/test.ts';

/** On narrow screens the saved filters sit behind a button. */
async function openSavedFilters(page: Page): Promise<void> {
  await expect(page.locator('[data-page="issues"][data-ready="true"]')).toBeVisible();
  const toggle = page.getByRole('button', { name: 'Filtros guardados' });
  if (await toggle.isVisible()) await toggle.click();
}

test.describe('navegador de incidencias', () => {
  test('consulta avanzada con error de validación y luego correcta', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/issues'));
    await page.getByRole('button', { name: 'Avanzado' }).click();

    const editor = page.getByRole('textbox', { name: 'Consulta avanzada' });
    await editor.fill('statu = todo');
    // The validation runs while typing: the problem is listed with its position and a suggested fix.
    const problems = page.getByTestId('coreql-problems');
    await expect(problems).toContainText("campo desconocido 'statu'");
    await expect(problems).toContainText('posición 1');
    await page.getByRole('button', { name: 'Usar «status»' }).click();
    await expect(editor).toHaveValue('status = todo');

    await editor.fill('statusCategory = todo ORDER BY created DESC');
    await page.getByTestId('run-query').click();
    await expect(page.getByTestId('result-count')).toContainText(/incidencia/);
    await expect(page.getByTestId('query-error')).toHaveCount(0);
    await expect(page).toHaveURL(/jql=statusCategory/);

    // The search survives a reload.
    await page.reload();
    await expect(page.getByRole('textbox', { name: 'Consulta avanzada' })).toHaveValue(
      'statusCategory = todo ORDER BY created DESC',
    );
  });

  test('los filtros básicos se convierten en chips y en la dirección', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/issues?status=todo,in_progress&page=1'));
    const chips = page.getByTestId('query-chip');
    await expect(chips).toHaveCount(2);
    await chips
      .first()
      .getByRole('button', { name: /Quitar filtro/ })
      .click();
    await expect(chips).toHaveCount(1);
    await expect(page).toHaveURL(/status=in_progress/);
  });

  test('guarda un filtro, lo recarga y lo comparte con otro rol', async ({ as, api, data }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const name = data.uniqueName('filtro');
    const page = await as('owner');
    await page.goto(appPath('/issues?jql=statusCategory%20%3D%20todo'));
    await page.getByTestId('save-filter').click();
    await page.getByTestId('filter-name').fill(name);
    await page.getByTestId('filter-save').click();
    await expect(page.getByTestId('current-filter')).toContainText(name);

    const listed = await owner.get('filters?scope=mine');
    const filters = ((await listed.json()) as { data: Array<{ id: string; name: string }> }).data;
    const saved = filters.find((filter) => filter.name === name);
    expect(saved, 'the saved filter is listed by the API').toBeTruthy();
    data.track('owner', `filters/${saved?.id}`);

    // After a reload the filter is still loaded from the address.
    await page.reload();
    await openSavedFilters(page);
    await expect(page.getByTestId('sidebar-filter').filter({ hasText: name })).toBeVisible();

    // Shared with the organization, another role sees it and can run it.
    const shared = await owner.put(`filters/${saved?.id}/shares`, {
      data: { shares: [{ type: 'organization', target_id: orgId, permission: 'view' }] },
    });
    expect(shared.ok()).toBe(true);
    const member = await as('member');
    await member.goto(appPath('/issues'));
    await openSavedFilters(member);
    const row = member.getByTestId('sidebar-filter').filter({ hasText: name });
    await expect(row).toBeVisible();
    await row.click();
    await expect(member.getByTestId('result-count')).toContainText(/incidencia/);
  });

  test('pasa la auditoría de maquetación en modo básico', async ({ as, audit }) => {
    const page = await as('member');
    await page.goto(appPath('/issues'));
    await expect(page.locator('[data-page="issues"][data-ready="true"]')).toBeVisible();
    await audit(page, { template: 'wide', name: 'issues-navigator' });
  });

  test('pasa la auditoría de maquetación en modo avanzado', async ({ as, audit }) => {
    const page = await as('member');
    await page.goto(appPath('/issues?jql=statusCategory%20%3D%20todo'));
    await expect(page.getByRole('textbox', { name: 'Consulta avanzada' })).toBeVisible();
    await audit(page, { template: 'wide', name: 'issues-navigator-advanced' });
  });
});

test.describe('página de filtros', () => {
  test('lista, filtra por ámbito y pasa la auditoría', async ({ as, api, data, audit }) => {
    const owner = await api('owner');
    const name = data.uniqueName('pagina');
    const created = await owner.post('filters', { data: { name, query: 'project = KAN' } });
    expect(created.ok()).toBe(true);
    const id = ((await created.json()) as { data: { id: string } }).data.id;
    data.track('owner', `filters/${id}`);

    const page = await as('owner');
    await page.goto(appPath('/filters'));
    await expect(page.getByTestId('filter-row-name').filter({ hasText: name })).toBeVisible();
    await page.getByRole('button', { name: 'Compartidos conmigo' }).click();
    await expect(page.getByTestId('filter-row-name').filter({ hasText: name })).toHaveCount(0);
    await page.getByRole('button', { name: 'Míos' }).click();
    await expect(page.getByTestId('filter-row-name').filter({ hasText: name })).toBeVisible();
    await audit(page, { template: 'wide', name: 'filters-page' });
  });
});

test.describe('búsqueda global', () => {
  test('resalta las coincidencias y ofrece todos los resultados', async ({ as }) => {
    const page = await as('owner');
    await page.goto(appPath('/dashboard'));
    await expect(page.locator('[data-page]').first()).toBeVisible();
    test.skip(
      !(await page.getByRole('searchbox', { name: 'Buscar proyectos e incidencias' }).isVisible()),
      'The header field is an icon on narrow screens',
    );
    const field = page.getByRole('searchbox', { name: 'Buscar proyectos e incidencias' });
    await field.fill('kanban');
    await expect(page.locator('.global-search__result mark').first()).toBeVisible();
    await expect(page.locator('.global-search__result mark').first()).toHaveText(/kan/i);
    await page.getByTestId('search-all').click();
    await expect(page).toHaveURL(/\/issues\?jql=text/);
  });

  test('la paleta agrupa resultados y abre el navegador con la consulta', async ({ as }) => {
    const page = await as('owner');
    await page.goto(appPath('/dashboard'));
    await expect(page.locator('[data-page]').first()).toBeVisible();
    await page.keyboard.press('Control+k');
    const input = page.getByRole('combobox', { name: 'Buscar o escribir un comando' });
    await input.fill('ana');
    await expect(page.getByRole('group', { name: 'Personas' })).toBeVisible();
    await page.getByRole('option', { name: /Ver todos los resultados/ }).click();
    await expect(page).toHaveURL(/jql=text/);
  });
});
