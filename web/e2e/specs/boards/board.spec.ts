import type { Page } from '@playwright/test';

import { expect, test, type DataHelpers } from '../../fixtures/test.ts';
import {
  boardViewViaApi,
  cardKeysByColumn,
  mainBoardId,
  setColumnLimitViaApi,
  setQuickFiltersViaApi,
} from '../../lib/agile-api.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createProjectViaApi, randomProjectKey, type ApiProject } from '../../lib/projects-api.ts';
import { createIssueViaApi, userIdOf } from '../../lib/workflow-api.ts';
import type { RoleKey } from '../../fixtures/roles.ts';

// Board of a Kanban project, end to end: moving cards (menu, keyboard and pointer), the WIP warning, quick
// filters, swimlanes, what a viewer can do and the settings drawer. The API prepares and verifies; the UI does
// what a person does. Data is created per test and removed with its project.

type ApiFor = (role?: RoleKey) => Promise<import('@playwright/test').APIRequestContext>;

async function kanbanProject(api: ApiFor, data: DataHelpers): Promise<ApiProject> {
  const owner = await api('owner');
  const acme = await orgIdBySlug(owner, 'acme');
  const project = await createProjectViaApi(
    owner,
    acme,
    data.uniqueName('brd'),
    randomProjectKey(),
  );
  data.trackProject('owner', project);
  return project;
}

function boardPath(project: ApiProject): string {
  return appPath(`/projects/${project.key}/board`);
}

async function openBoard(page: Page, project: ApiProject): Promise<void> {
  await page.goto(boardPath(project));
  await expect(page.locator('[data-column]').first()).toBeVisible();
}

function card(page: Page, key: string) {
  return page.getByTestId(`kanban-card-${key}`);
}

/** Moves a card with the pointer: press on it, carry it over the cell of another column and release. */
async function dragCardToColumn(page: Page, key: string, columnIndex: number): Promise<void> {
  const source = await card(page, key).boundingBox();
  const target = await page.locator('[data-column]').nth(columnIndex).boundingBox();
  if (!source || !target) throw new Error('card or column is not visible');
  await page.mouse.move(source.x + 24, source.y + 24);
  await page.mouse.down();
  await page.mouse.move(source.x + 48, source.y + 48, { steps: 4 });
  await page.mouse.move(target.x + target.width / 2, target.y + 40, { steps: 14 });
  await page.mouse.up();
}

test.describe('tablero kanban', () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(testInfo.project.name !== 'desktop', 'El flujo se ejecuta en el proyecto desktop');
  });

  test('mover una tarjeta con el menú, con el teclado y con el puntero', async ({
    as,
    api,
    data,
  }) => {
    const project = await kanbanProject(api, data);
    const owner = await api('owner');
    const first = await createIssueViaApi(owner, project.id, 'Primera tarjeta');
    const second = await createIssueViaApi(owner, project.id, 'Segunda tarjeta');
    const third = await createIssueViaApi(owner, project.id, 'Tercera tarjeta');
    const boardId = await mainBoardId(owner, project.id);

    const page = await as('owner');
    await openBoard(page, project);

    // Menu: the keyboard and touch alternative lists the other columns.
    await page.getByRole('button', { name: `Mover ${first.key} a otra columna` }).click();
    await page.getByRole('menuitem').first().click();
    await expect(
      page.getByRole('status').filter({ hasText: `${first.key} movida a` }),
    ).toBeVisible();
    await expect.poll(async () => (await cardKeysByColumn(owner, boardId))[1]).toContain(first.key);

    // Keyboard: Alt+ArrowRight on the card moves it to the next column.
    await card(page, second.key).getByRole('link').focus();
    await page.keyboard.press('Alt+ArrowRight');
    await expect
      .poll(async () => (await cardKeysByColumn(owner, boardId))[1])
      .toContain(second.key);

    // Pointer: drag the third card over the second column.
    await dragCardToColumn(page, third.key, 1);
    await expect.poll(async () => (await cardKeysByColumn(owner, boardId))[1]).toContain(third.key);
    await expect(
      page.locator('[data-column]').nth(1).getByTestId(`kanban-card-${third.key}`),
    ).toBeVisible();
  });

  test('superar el límite WIP avisa y no impide mover', async ({ as, api, data }) => {
    const project = await kanbanProject(api, data);
    const owner = await api('owner');
    const boardId = await mainBoardId(owner, project.id);
    await setColumnLimitViaApi(owner, boardId, 1, 1);
    const first = await createIssueViaApi(owner, project.id, 'Dentro del límite');
    const second = await createIssueViaApi(owner, project.id, 'Fuera del límite');

    const page = await as('owner');
    await openBoard(page, project);
    for (const created of [first, second]) {
      await page.getByRole('button', { name: `Mover ${created.key} a otra columna` }).click();
      await page.getByRole('menuitem').first().click();
      await expect
        .poll(async () => (await cardKeysByColumn(owner, boardId))[1])
        .toContain(created.key);
    }

    await expect(page.getByTestId('wip-warning')).toContainText('límite');
    await expect(page.locator('[data-column]').nth(1)).toHaveAttribute('data-wip', 'over_max');
    await expect(page.getByTestId('column-count').nth(1)).toContainText('2/1');
  });

  test('un filtro rápido deja solo las tarjetas que cumplen la consulta', async ({
    as,
    api,
    data,
  }) => {
    const project = await kanbanProject(api, data);
    const owner = await api('owner');
    const boardId = await mainBoardId(owner, project.id);
    await setQuickFiltersViaApi(owner, boardId, [
      { name: 'Mis incidencias', query: 'assignee = currentUser()' },
    ]);
    const mine = await createIssueViaApi(owner, project.id, 'Es mía', {
      assignee_id: await userIdOf(owner),
    });
    const other = await createIssueViaApi(owner, project.id, 'Sin responsable');

    const page = await as('owner');
    await openBoard(page, project);
    await expect(card(page, mine.key)).toBeVisible();
    await expect(card(page, other.key)).toBeVisible();

    const chip = page.getByTestId('quick-filter-Mis incidencias');
    await chip.click();
    await expect(chip).toHaveAttribute('aria-pressed', 'true');
    await expect(card(page, mine.key)).toBeVisible();
    await expect(card(page, other.key)).toHaveCount(0);

    await page.getByTestId('quick-filters-clear').click();
    await expect(card(page, other.key)).toBeVisible();
  });

  test('agrupar por responsable crea un carril por persona y se puede plegar', async ({
    as,
    api,
    data,
  }) => {
    const project = await kanbanProject(api, data);
    const owner = await api('owner');
    const boardId = await mainBoardId(owner, project.id);
    const mine = await createIssueViaApi(owner, project.id, 'Con responsable', {
      assignee_id: await userIdOf(owner),
    });
    const loose = await createIssueViaApi(owner, project.id, 'Sin dueño');

    const page = await as('owner');
    await openBoard(page, project);
    await page.getByTestId('swimlane-select').click();
    await page.getByRole('option', { name: 'Por responsable' }).click();

    await expect(page.locator('.board-lane__toggle')).toHaveCount(2);
    await expect(card(page, mine.key)).toBeVisible();
    await expect
      .poll(async () => (await boardViewViaApi(owner, boardId)).board.swimlane_mode)
      .toBe('assignee');

    const toggle = page.locator('.board-lane__toggle').filter({ hasText: 'Sin asignar' });
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await expect(card(page, loose.key)).toHaveCount(0);
    await toggle.click();
    await expect(card(page, loose.key)).toBeVisible();
  });

  test('quien solo observa no puede arrastrar, mover ni crear', async ({ as, api, data }) => {
    const project = await kanbanProject(api, data);
    const owner = await api('owner');
    const viewer = await api('viewer');
    const added = await owner.post(`projects/${project.id}/members`, {
      data: { user_id: await userIdOf(viewer), role: 'viewer' },
    });
    expect(added.ok(), 'adding the viewer').toBeTruthy();
    const issue = await createIssueViaApi(owner, project.id, 'Solo lectura');

    const page = await as('viewer');
    await openBoard(page, project);
    await expect(card(page, issue.key)).toBeVisible();
    await expect(page.getByRole('button', { name: /^Mover .* a otra columna$/ })).toHaveCount(0);
    await expect(page.getByTestId('board-create-open')).toHaveCount(0);
    await expect(page.getByTestId('swimlane-select')).toHaveCount(0);
    await expect(page.locator('.board-cell__item--draggable')).toHaveCount(0);
  });

  test('el lead edita el límite de una columna desde la configuración del tablero', async ({
    as,
    api,
    data,
  }) => {
    const project = await kanbanProject(api, data);
    const owner = await api('owner');
    const boardId = await mainBoardId(owner, project.id);

    const page = await as('owner');
    await openBoard(page, project);
    await page.getByTestId('board-more').click();
    await page.getByTestId('board-settings').click();
    const drawer = page.getByTestId('board-settings-drawer');
    await expect(drawer).toBeVisible();

    await drawer.getByTestId('column-max-1').fill('3');
    await drawer.getByTestId('save-columns').click();
    await expect
      .poll(async () => (await boardViewViaApi(owner, boardId)).columns[1].max_issues)
      .toBe(3);
    await expect(page.getByTestId('column-count').nth(1)).toContainText('/3');

    await drawer.getByRole('tab', { name: 'Filtros rápidos' }).click();
    await drawer.getByTestId('add-filter').click();
    await drawer.getByTestId('filter-name-0').fill('Urgentes');
    await drawer.getByTestId('filter-query-0').fill('priority in (Alta, Máxima)');
    await drawer.getByTestId('save-filters').click();
    await expect(page.getByTestId('quick-filter-Urgentes')).toBeVisible();
  });
});

test.describe('diseño del tablero', () => {
  test('las columnas llenan el ancho y no sobra espacio muerto', async ({
    as,
    api,
    data,
    audit,
  }) => {
    const project = await kanbanProject(api, data);
    const owner = await api('owner');
    for (const summary of ['Una', 'Dos', 'Tres']) {
      await createIssueViaApi(owner, project.id, `Tarjeta ${summary}`);
    }

    const page = await as('owner');
    await openBoard(page, project);
    await expect(page.getByText('Tarjeta Una')).toBeVisible();

    const viewport = page.viewportSize();
    const headers = page.locator('app-board-column-header');
    const count = await headers.count();
    const last = await headers.nth(count - 1).boundingBox();
    const scroller = await page.locator('.project-board__scroller').boundingBox();
    if (!last || !scroller || !viewport) throw new Error('the board is not rendered');
    if (viewport.width >= 768) {
      // With a few columns they share the width: the last one ends where the board ends.
      expect(last.x + last.width).toBeGreaterThan(scroller.x + scroller.width - 24);
    }

    await audit(page, { template: 'fluid', name: 'board' });
  });
});
