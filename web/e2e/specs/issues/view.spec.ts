import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';
import { openIssueView, projectWithIssue, readView } from './issue-fixtures.ts';

test.describe('vista de la incidencia', () => {
  test('quien lidera edita puntos, etiquetas y marca, y los cambios quedan guardados', async ({
    as,
    api,
    data,
  }) => {
    const { owner, issue } = await projectWithIssue(api, data, 'view');

    const page = await as('lead');
    await openIssueView(page, issue.key);

    // Story points through the inline editor.
    await page
      .getByRole('button', { name: /Puntos de historia/ })
      .first()
      .click();
    await page.getByRole('textbox', { name: /Puntos de historia/ }).fill('5');
    await page.keyboard.press('Enter');
    await expect(page.getByText('Puntos actualizados').first()).toBeVisible();

    // Labels are typed as free text; the server creates the missing ones.
    const labels = page.getByRole('textbox', { name: /Etiquetas/ });
    await labels.fill('e2e-uno, e2e-dos');
    await labels.blur();
    await expect(page.getByText('Etiquetas actualizadas').first()).toBeVisible();

    // The flag.
    await page.getByRole('checkbox', { name: 'Incidencia marcada' }).check();
    await expect(page.getByText('Marca actualizada').first()).toBeVisible();

    const view = await readView(owner, issue.key);
    expect(view['story_points']).toBe(5);
    expect(view['flagged']).toBe(true);
    const search = await owner.get('issues/search', {
      params: { jql: `key = "${issue.key}"`, fields: 'labels', page_size: '1', total: 'false' },
    });
    const found = ((await search.json()) as { data: { labels: string[] }[] }).data[0];
    expect([...found.labels].sort()).toEqual(['e2e-dos', 'e2e-uno']);
  });

  test('quien solo observa ve los campos sin controles de edición ni registro de tiempo', async ({
    as,
    api,
    data,
  }) => {
    const { issue } = await projectWithIssue(api, data, 'viewro');
    const page = await as('viewer');
    await openIssueView(page, issue.key);
    await expect(page.getByRole('textbox', { name: /Etiquetas/ })).toHaveCount(0);
    await expect(page.getByTestId('log-work-open')).toHaveCount(0);
    await expect(page.getByTestId('link-open')).toHaveCount(0);
    await expect(page.getByRole('checkbox', { name: 'Incidencia marcada' })).toBeDisabled();
  });

  test('una edición obsoleta se rechaza y se cargan los datos actuales', async ({
    as,
    api,
    data,
  }) => {
    const { owner, issue } = await projectWithIssue(api, data, 'stale');
    const page = await as('lead');
    await openIssueView(page, issue.key);

    // Someone else changes the issue after the page loaded.
    const other = await owner.patch(`issues/${issue.id}`, { data: { story_points: 8 } });
    expect(other.ok()).toBe(true);

    await page.getByRole('checkbox', { name: 'Incidencia marcada' }).click();
    await expect(page.getByText(/Alguien cambió esta incidencia/).first()).toBeVisible();
    await expect(page.getByRole('checkbox', { name: 'Incidencia marcada' })).not.toBeChecked();
  });

  test('subtarea rápida, vínculo, voto y clonado desde la vista', async ({ as, api, data }) => {
    const { owner, project, issue } = await projectWithIssue(api, data, 'rel');
    const targetKey = (await createIssueViaApi(owner, project.id, 'Destino del vínculo')).key;

    const page = await as('lead');
    await page.goto(appPath(`/browse/${issue.key}`));

    // Quick subtask.
    await page.getByRole('textbox', { name: 'Resumen de la nueva subtarea' }).fill('Subtarea e2e');
    await page.getByRole('button', { name: 'Agregar subtarea' }).click();
    await expect(page.getByTestId('child-item').filter({ hasText: 'Subtarea e2e' })).toBeVisible();
    await expect(page.getByTestId('issue-children').locator('app-progress-bar')).toBeVisible();

    // Link with a kind and an issue search.
    await page.getByTestId('link-open').click();
    await page.getByRole('combobox', { name: 'Tipo de vínculo' }).click();
    await page.getByRole('option', { name: 'Bloquea', exact: true }).click();
    await page.getByRole('combobox', { name: 'Incidencia a vincular' }).fill('Destino');
    await page.getByRole('option', { name: new RegExp(targetKey) }).click();
    await page.getByTestId('link-submit').click();
    const link = page.getByTestId('link-item').filter({ hasText: targetKey });
    await expect(link).toBeVisible();
    await expect(page.getByRole('heading', { name: 'bloquea', level: 3 })).toBeVisible();

    // Vote.
    await page.getByTestId('issue-vote').click();
    await expect(page.getByTestId('issue-vote')).toContainText('1');

    // Clone from the actions menu.
    await page.getByRole('button', { name: 'Más acciones' }).click();
    await page.getByRole('menuitem', { name: 'Clonar' }).click();
    await page.getByTestId('operation-submit').click();
    await expect(page).toHaveURL(/\/browse\/[A-Z0-9]+-\d+$/);
    await expect(page.getByRole('heading', { level: 1 })).toContainText('Copia de');
  });

  test('la vista y la creación no rompen el diseño', async ({ as, api, data, audit }) => {
    const { issue } = await projectWithIssue(api, data, 'audit');
    const page = await as('lead');
    await openIssueView(page, issue.key);
    await audit(page, { template: 'wide', name: 'issue-view' });
  });
});
