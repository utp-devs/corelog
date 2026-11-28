import { readFile, writeFile } from 'node:fs/promises';

import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';

interface IssueRow {
  readonly key: string;
  readonly summary: string;
}

test.describe('importar y exportar', () => {
  test('importa un CSV: sube, mapea, revisa, importa y verifica las incidencias creadas', async ({
    as,
    api,
    data,
  }, testInfo) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'imp');
    const tag = data.uniqueName('imp');
    const csv = [
      'Titulo,Prioridad,Etiquetas',
      `${tag} uno,High,a;b`,
      `${tag} dos,Low,c`,
      ',Medium,sin-titulo',
    ].join('\n');
    const file = testInfo.outputPath('incidencias.csv');
    await writeFile(file, csv, 'utf8');

    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/import`));
    await expect(page.getByTestId('import-drop')).toBeVisible();

    // Files over the limit or of another kind are refused before any request.
    await page.getByTestId('import-file').setInputFiles({
      name: 'datos.xlsx',
      mimeType: 'application/octet-stream',
      buffer: Buffer.from('x'),
    });
    await expect(page.getByTestId('import-error')).toContainText('CSV');

    await page.getByTestId('import-file').setInputFiles(file);
    await expect(page.getByTestId('import-step-map')).toBeVisible();
    const mapping = page.getByTestId('mapping-table');
    await expect(mapping).toContainText('Titulo');

    // The summary is mandatory: map it and review.
    const review = page.getByTestId('import-review').getByRole('button');
    await page.getByTestId('mapping-select-0').selectOption('summary');
    await page.getByTestId('mapping-select-1').selectOption('priority');
    await page.getByTestId('mapping-select-2').selectOption('labels');
    await expect(review).toBeEnabled();
    await review.click();

    await expect(page.getByTestId('import-step-review')).toBeVisible({ timeout: 60_000 });
    await expect(page.getByTestId('dry-total')).toHaveText('3');
    await expect(page.getByTestId('dry-errors')).toHaveText('1');
    await expect(page.getByTestId('dry-error-table')).toContainText('3');

    // The error report downloads as a CSV.
    const download = page.waitForEvent('download');
    await page.getByTestId('import-errors-csv').getByRole('button').click();
    const report = await download;
    expect(report.suggestedFilename()).toMatch(/\.csv$/);
    const content = await readFile((await report.path()) ?? '', 'utf8');
    expect(content.length).toBeGreaterThan(5);

    await page.getByTestId('import-start').getByRole('button').click();
    await expect(page.getByTestId('import-created')).toHaveText('2', { timeout: 60_000 });
    await expect(page.getByTestId('import-error-rows')).toHaveText('1');
    await expect(page.getByTestId('import-open-issues')).toBeVisible();

    // The issues exist in the project.
    const found = await owner.get('issues', { params: { project_id: project.id } });
    const issues = ((await found.json()) as { data: IssueRow[] | { items: IssueRow[] } }).data;
    const list = Array.isArray(issues) ? issues : issues.items;
    expect(list.filter((issue) => issue.summary.startsWith(tag))).toHaveLength(2);
  });

  test('exporta las incidencias en CSV con filtro y la copia completa en JSON', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'exp');
    const summary = data.uniqueName('exportada');
    const created = await owner.post('issues', {
      data: { project_id: project.id, summary, description: 'e2e' },
    });
    expect(created.status()).toBe(201);

    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/export`));
    await expect(page.getByTestId('export-csv')).toBeVisible();

    const csvDownload = page.waitForEvent('download');
    await page.getByTestId('export-csv').getByRole('button').click();
    const csv = await csvDownload;
    expect(csv.suggestedFilename()).toMatch(/\.csv$/);
    const text = await readFile((await csv.path()) ?? '', 'utf8');
    expect(text).toContain(summary);
    await expect(page.getByTestId('export-csv-done')).toBeVisible();

    const jsonDownload = page.waitForEvent('download');
    await page.getByTestId('export-json').getByRole('button').click();
    const json = await jsonDownload;
    expect(json.suggestedFilename()).toMatch(/\.json$/);
    const body = await readFile((await json.path()) ?? '', 'utf8');
    expect(body).toContain(summary);
    await expect(page.getByTestId('export-json-done')).toBeVisible();
  });

  test('un filtro CoreQL no válido se explica en español', async ({ as, api, data }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'exp');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/export`));
    await page
      .getByRole('combobox', { name: 'Filtro CoreQL de la exportación' })
      .fill('esto no es ((');
    await page.getByTestId('export-csv').getByRole('button').click();
    await expect(page.getByTestId('export-csv-error')).toBeVisible();
  });

  test('las páginas cumplen la auditoría de layout', async ({ as, api, data, audit }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'imp');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/import`));
    await expect(page.getByTestId('import-drop')).toBeVisible();
    await audit(page, { template: 'wide', name: 'import' });
    await page.goto(appPath(`/projects/${project.key}/settings/export`));
    await expect(page.getByTestId('export-csv')).toBeVisible();
    await audit(page, { template: 'wide', name: 'export' });
  });
});
