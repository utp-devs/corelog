import { readFile } from 'node:fs/promises';

import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

// Reports of the seeded Scrum project (SCRUM): the hub, every report, the range in the URL, the CSV export
// and the layout of the screens. The seeded project has an active sprint, issues and history.

const HUB = appPath('/projects/SCRUM/reports');

const KINDS = [
  { kind: 'burndown', title: 'Burndown del sprint' },
  { kind: 'burnup', title: 'Burnup del sprint' },
  { kind: 'velocity', title: 'Velocidad' },
  { kind: 'cumulative-flow', title: 'Flujo acumulado' },
  { kind: 'sprint-report', title: 'Resumen del sprint' },
  { kind: 'control', title: 'Gráfico de control' },
  { kind: 'created-vs-resolved', title: 'Creadas frente a resueltas' },
  { kind: 'resolution-time', title: 'Tiempo de resolución' },
  { kind: 'time-tracking', title: 'Seguimiento de tiempo' },
  { kind: 'workload', title: 'Carga de trabajo' },
  { kind: 'version', title: 'Versión' },
] as const;

test.describe('reportes del proyecto', () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(
      testInfo.project.name !== 'desktop',
      'El flujo funcional se ejecuta en el proyecto desktop',
    );
  });

  test('lead abre el hub con los once reportes y cada uno carga sin error', async ({ as }) => {
    test.setTimeout(180_000);
    const page = await as('lead');
    await page.goto(HUB);
    await expect(page.getByTestId('reports-hub')).toBeVisible();
    for (const { kind, title } of KINDS) {
      await expect(page.getByTestId(`report-card-${kind}`)).toContainText(title);
    }
    await expect(page.getByRole('heading', { name: 'Ágil' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Tiempo y flujo' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Versiones' })).toBeVisible();

    for (const { kind, title } of KINDS) {
      await page.goto(appPath(`/projects/SCRUM/reports/${kind}`));
      await expect(page.locator('[data-page="report"][data-ready="true"]')).toBeVisible();
      await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
      await expect(page.getByTestId('report-error')).toHaveCount(0);
      await expect(page.getByTestId('report-params')).toBeVisible();
    }
  });

  test('abrir una tarjeta lleva al reporte y se puede volver al hub', async ({ as }) => {
    const page = await as('lead');
    await page.goto(HUB);
    await page.getByTestId('report-card-workload').click();
    await expect(page).toHaveURL(/\/projects\/SCRUM\/reports\/workload$/);
    await expect(page.getByTestId('report-table')).toBeVisible();
    await expect(page.getByTestId('report-chart').getByRole('img')).toHaveAttribute(
      'aria-label',
      /Los datos completos están en la tabla/,
    );
    await page.getByTestId('report-back').click();
    await expect(page.getByTestId('reports-hub')).toBeVisible();
  });

  test('cambiar el rango se guarda en la URL y sobrevive a recargar', async ({ as }) => {
    const page = await as('lead');
    await page.goto(appPath('/projects/SCRUM/reports/created-vs-resolved'));
    await expect(page.getByTestId('report-preset-30')).toHaveAttribute('aria-pressed', 'true');

    await page.getByTestId('report-preset-7').click();
    await expect(page).toHaveURL(/from=\d{4}-\d{2}-\d{2}&to=\d{4}-\d{2}-\d{2}/);
    await expect(page.getByTestId('report-preset-7')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('[data-testid="report-table"] tbody tr')).toHaveCount(7);

    await page.reload();
    await expect(page.getByTestId('report-preset-7')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('[data-testid="report-table"] tbody tr')).toHaveCount(7);
  });

  test('un rango demasiado grande muestra el error en español', async ({ as }) => {
    const page = await as('lead');
    await page.goto(
      appPath('/projects/SCRUM/reports/created-vs-resolved?from=2020-01-01&to=2026-11-01'),
    );
    await expect(page.getByTestId('report-error')).toContainText('366 días');
    await expect(page.getByTestId('report-params')).toBeVisible();
  });

  test('exportar CSV descarga un archivo con BOM y cabeceras en español', async ({ as }) => {
    const page = await as('lead');
    await page.goto(appPath('/projects/SCRUM/reports/workload'));
    await expect(page.getByTestId('report-table')).toBeVisible();

    const download = page.waitForEvent('download');
    await page.getByTestId('report-export').getByRole('button').click();
    const file = await download;
    expect(file.suggestedFilename()).toMatch(/^SCRUM-workload-\d{4}-\d{2}-\d{2}\.csv$/);
    const content = await readFile(await file.path(), 'utf8');
    expect(content.charCodeAt(0)).toBe(0xfeff);
    expect(content).toContain('Persona');
    expect(content).toContain('Abiertas');
  });

  test('ordenar la tabla por una columna invierte el orden de las filas', async ({ as }) => {
    const page = await as('lead');
    await page.goto(appPath('/projects/SCRUM/reports/workload'));
    const rows = page.locator('[data-testid="report-table"] tbody tr td:first-child');
    await expect(rows.first()).toBeVisible();
    await page.getByTestId('report-sort-assignee').click();
    const ascending = (await rows.allTextContents()).map((text) => text.trim());
    await page.getByTestId('report-sort-assignee').click();
    const descending = (await rows.allTextContents()).map((text) => text.trim());
    expect(descending).toEqual([...ascending].reverse());
  });

  test('viewer ve los reportes permitidos y abre uno', async ({ as }) => {
    const page = await as('viewer');
    await page.goto(HUB);
    await expect(page.getByTestId('reports-hub')).toBeVisible();
    await expect(page.getByTestId('report-card-workload')).toBeVisible();
    await page.getByTestId('report-card-workload').click();
    await expect(page.getByTestId('report-table')).toBeVisible();
  });

  test('un reporte que no existe muestra la página de no encontrado', async ({ as }) => {
    const page = await as('lead');
    await page.goto(appPath('/projects/SCRUM/reports/no-existe'));
    await expect(page.locator('app-error-page')).toBeVisible();
  });
});

test.describe('diseño de los reportes', () => {
  test('el hub llena el ancho sin espacio muerto', async ({ as, audit }) => {
    const page = await as('lead');
    await page.goto(HUB);
    await expect(page.getByTestId('reports-hub')).toBeVisible();
    await audit(page, { template: 'wide', name: 'reports-hub' });
  });

  for (const kind of ['burndown', 'created-vs-resolved']) {
    test(`el reporte ${kind} llena el ancho con gráfico y tabla`, async ({ as, audit }) => {
      const page = await as('lead');
      await page.goto(appPath(`/projects/SCRUM/reports/${kind}`));
      await expect(page.getByTestId('report-chart')).toBeVisible();
      await expect(page.getByTestId('report-table')).toBeVisible();

      const chart = await page.getByTestId('report-chart').boundingBox();
      const table = await page.getByTestId('report-table').boundingBox();
      const params = await page.getByTestId('report-params').boundingBox();
      if (!chart || !table || !params) throw new Error('the report is not rendered');
      // The blocks share the same column: the chart and the parameters have the same width.
      expect(Math.abs(chart.width - params.width)).toBeLessThan(2);
      expect(table.width).toBeGreaterThan(chart.width - 80);

      await audit(page, { template: 'wide', name: `report-${kind}` });
    });
  }
});
