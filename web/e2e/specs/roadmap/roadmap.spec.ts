import type { APIRequestContext } from '@playwright/test';

import { expect, test, type DataHelpers } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';

// Roadmap of a project: bars of epics on a timeline, items without dates, grouping, order and table view.
// The API prepares the epics (dated relative to today so the bars are always inside the default range).

interface Prepared {
  readonly project: { id: string; key: string };
  readonly owner: APIRequestContext;
  readonly a: { id: string; key: string };
  readonly b: { id: string; key: string };
  readonly undated: { id: string; key: string };
}

function isoDay(offset: number): string {
  const date = new Date();
  date.setDate(date.getDate() + offset);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

async function prepare(
  api: (role?: never) => Promise<APIRequestContext>,
  data: DataHelpers,
): Promise<Prepared> {
  const owner = await (api as (role: 'owner') => Promise<APIRequestContext>)('owner');
  const acme = await orgIdBySlug(owner, 'acme');
  const project = await data.createProject('owner', acme, 'road');
  const epic = (summary: string, extra: Record<string, unknown> = {}) =>
    createIssueViaApi(owner, project.id, summary, { issue_type: 'epic', ...extra });
  const a = await epic('Epica Autenticacion', { start_date: isoDay(-10), due_date: isoDay(20) });
  const b = await epic('Epica Facturacion', { start_date: isoDay(15), due_date: isoDay(60) });
  const undated = await epic('Epica Sin Fechas');
  const link = await owner.post(`issues/${a.id}/links`, { data: { target: b.id, kind: 'blocks' } });
  expect(link.ok(), await link.text()).toBe(true);
  const version = await owner.post(`projects/${project.id}/versions`, {
    data: { name: 'v1.0', start_date: isoDay(0), release_date: isoDay(45) },
  });
  expect(version.ok(), await version.text()).toBe(true);
  return { project, owner, a, b, undated };
}

function roadmapPath(key: string): string {
  return appPath(`/projects/${key}/roadmap`);
}

test.describe('hoja de ruta', () => {
  test('pinta las barras de las epicas, la linea de hoy y las dependencias', async ({
    as,
    api,
    data,
    audit,
  }) => {
    const { project, a, b, undated } = await prepare(api, data);
    const page = await as('owner');
    await page.goto(roadmapPath(project.key));
    await expect(page.locator('[data-page="roadmap"][data-ready="true"]')).toBeVisible();

    await expect(page.getByRole('heading', { name: 'Hoja de ruta', level: 1 })).toBeVisible();
    // Phones get a list with progress bars instead of the timeline.
    if (test.info().project.name.startsWith('mobile')) {
      await expect(page.getByTestId(`roadmap-card-${a.id}`)).toBeVisible();
      await expect(page.getByTestId(`roadmap-card-${b.id}`)).toBeVisible();
      await audit(page, { template: 'wide', name: 'roadmap-phone' });
      return;
    }
    await expect(page.getByTestId(`timeline-bar-${a.id}`)).toBeVisible();
    await expect(page.getByTestId(`timeline-bar-${b.id}`)).toBeVisible();
    await expect(page.getByTestId(`timeline-bar-${undated.id}`)).toHaveCount(0);
    await expect(page.getByTestId('timeline-today')).toBeVisible();
    await expect(page.getByTestId('timeline-dependency')).toHaveCount(1);
    await expect(page.getByTestId('timeline-band').first()).toContainText('v1.0');

    // Items without dates are listed, not hidden.
    const undatedSection = page.getByTestId('roadmap-undated');
    await expect(undatedSection).toContainText('Sin fechas (1)');
    await expect(undatedSection).toContainText('Epica Sin Fechas');

    await audit(page, { template: 'wide', name: 'roadmap' });
  });

  test.describe('flujos', () => {
    test.beforeEach(({}, testInfo) => {
      test.skip(
        testInfo.project.name !== 'desktop',
        'Los flujos se ejecutan en el proyecto desktop',
      );
    });

    test('cambiar la escala y agrupar por versiones', async ({ as, api, data }) => {
      const { project } = await prepare(api, data);
      const page = await as('owner');
      await page.goto(roadmapPath(project.key));
      await expect(page.getByTestId('roadmap-zoom-month')).toHaveAttribute('aria-pressed', 'true');

      const monthWidth = (await page.locator('.timeline__canvas').boundingBox())?.width ?? 0;
      await page.getByTestId('roadmap-zoom-quarter').click();
      await expect(page.getByTestId('roadmap-zoom-quarter')).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      const quarterWidth = (await page.locator('.timeline__canvas').boundingBox())?.width ?? 0;
      expect(quarterWidth).toBeLessThan(monthWidth);

      await page.getByTestId('roadmap-group-version').click();
      await expect(page.getByTestId('roadmap-group-version')).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      await expect(page.locator('.timeline__title', { hasText: 'v1.0' })).toBeVisible();
    });

    test('filtrar por estado y buscar', async ({ as, api, data }) => {
      const { project, a } = await prepare(api, data);
      const page = await as('owner');
      await page.goto(roadmapPath(project.key));
      await expect(page.getByTestId(`timeline-row-${a.id}`)).toBeVisible();

      await page.getByTestId('roadmap-filter-done').click();
      await expect(page.getByText('Nada coincide con los filtros')).toBeVisible();
      await page.getByTestId('roadmap-filter-all').click();

      await page.getByRole('textbox', { name: 'Buscar en la hoja de ruta' }).fill('Facturacion');
      await expect(page.locator('.timeline__title')).toHaveCount(1);
      await expect(page.locator('.timeline__title')).toContainText('Facturacion');
    });

    test('reordenar las epicas con los botones y guardar el orden', async ({ as, api, data }) => {
      const { project, owner, a, b } = await prepare(api, data);
      const page = await as('owner');
      await page.goto(roadmapPath(project.key));
      await expect(page.getByTestId(`timeline-row-${a.id}`)).toBeVisible();

      const titles = () => page.locator('.timeline__title').allTextContents();
      expect((await titles())[0]).toContain('Autenticacion');
      await page.getByRole('button', { name: 'Bajar Epica Autenticacion' }).click();
      await expect.poll(async () => (await titles())[0]).toContain('Facturacion');

      await expect
        .poll(async () => {
          const response = await owner.get(`projects/${project.id}/roadmap`);
          const body = (await response.json()) as { data: { bars: Array<{ id: string }> } };
          return body.data.bars.map((bar) => bar.id).slice(0, 2);
        })
        .toEqual([b.id, a.id]);

      await page.reload();
      await expect(page.locator('.timeline__title').first()).toContainText('Facturacion');
    });

    test('la vista de tabla muestra todos los elementos, tambien los sin fechas', async ({
      as,
      api,
      data,
    }) => {
      const { project } = await prepare(api, data);
      const page = await as('owner');
      await page.goto(roadmapPath(project.key));
      await page.getByTestId('roadmap-view-table').click();

      const table = page.getByTestId('roadmap-table');
      await expect(table).toBeVisible();
      await expect(table.locator('tbody tr')).toHaveCount(3);
      await expect(table).toContainText('Epica Sin Fechas');
      await expect(table).toContainText('Sin fecha');
      await expect(page.locator('app-timeline')).toHaveCount(0);
    });

    test('abrir una epica desde la hoja de ruta lleva a su incidencia', async ({
      as,
      api,
      data,
    }) => {
      const { project, a } = await prepare(api, data);
      const page = await as('owner');
      await page.goto(roadmapPath(project.key));
      await page.getByTestId(`timeline-row-${a.id}`).click();
      await expect(page).toHaveURL(new RegExp(`/browse/${a.key}`));
    });

    test('un rol sin permiso para editar no puede reordenar', async ({ as, api, data }) => {
      const { project, a } = await prepare(api, data);
      const page = await as('viewer');
      await page.goto(roadmapPath(project.key));
      await expect(page.getByTestId(`timeline-row-${a.id}`)).toBeVisible();
      await expect(page.getByRole('button', { name: /^Bajar / })).toHaveCount(0);
    });

    test('un proyecto sin epicas muestra la ilustracion del modulo', async ({ as, api, data }) => {
      const owner = await api('owner');
      const acme = await orgIdBySlug(owner, 'acme');
      const project = await data.createProject('owner', acme, 'vacia');
      const page = await as('owner');
      await page.goto(roadmapPath(project.key));
      await expect(page.getByText('Aún no hay épicas')).toBeVisible();
      await expect(page.locator('img[src="/images/modules/module-roadmap.webp"]')).toBeVisible();
    });
  });
});
