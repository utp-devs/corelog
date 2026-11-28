import type { Page } from '@playwright/test';

import { expect, test, type DataHelpers } from '../../fixtures/test.ts';
import { listSprintsViaApi } from '../../lib/agile-api.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';
import type { RoleKey } from '../../fixtures/roles.ts';

// Backlog and sprint lifecycle of a Scrum project, end to end: create a sprint, plan issues into it, start it,
// complete it choosing where the unfinished work goes, and open its report. The API prepares and verifies.

type ApiFor = (role?: RoleKey) => Promise<import('@playwright/test').APIRequestContext>;

async function scrumProject(api: ApiFor, data: DataHelpers) {
  const owner = await api('owner');
  const acme = await orgIdBySlug(owner, 'acme');
  return data.createProject('owner', acme, 'bkl');
}

function backlogPath(key: string): string {
  return appPath(`/projects/${key}/backlog`);
}

function row(page: Page, key: string) {
  return page.getByTestId(`backlog-row-${key}`);
}

/** Opens the "Mover" menu of a row and sends it to a section by name. */
async function moveRowTo(page: Page, key: string, destination: string): Promise<void> {
  await page.getByRole('button', { name: `Mover ${key}`, exact: true }).click();
  await page.getByRole('menuitem', { name: destination, exact: true }).click();
}

async function createSprintViaUi(page: Page): Promise<void> {
  await page.getByTestId('create-sprint').getByRole('button').click();
  await expect(page.getByRole('dialog', { name: 'Crear sprint' })).toBeVisible();
  await page.getByTestId('sprint-confirm').getByRole('button').click();
}

test.describe('backlog y sprints', () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(testInfo.project.name !== 'desktop', 'El flujo se ejecuta en el proyecto desktop');
  });

  test('crear un sprint, planificar, iniciarlo, completarlo con destino y abrir su informe', async ({
    as,
    api,
    data,
  }) => {
    const project = await scrumProject(api, data);
    const owner = await api('owner');
    const done = await createIssueViaApi(owner, project.id, 'Se termina', { story_points: 3 });
    const open = await createIssueViaApi(owner, project.id, 'Queda pendiente', { story_points: 5 });
    await createIssueViaApi(owner, project.id, 'Se queda en el backlog');

    const page = await as('owner');
    await page.goto(backlogPath(project.key));
    await expect(row(page, done.key)).toBeVisible();

    // Create a sprint: a dialog proposes name and dates.
    await createSprintViaUi(page);
    await expect(page.locator('.backlog-section__title', { hasText: 'Sprint 1' })).toBeVisible();
    const planned = (await listSprintsViaApi(owner, project.id)).find(
      (s) => s.status === 'planned',
    );
    expect(planned, 'the sprint exists').toBeDefined();

    // Plan two issues into it.
    await moveRowTo(page, done.key, 'Sprint 1');
    await expect(
      page.getByTestId(`backlog-section-${planned?.id}`).getByTestId(`backlog-row-${done.key}`),
    ).toBeVisible();
    await moveRowTo(page, open.key, 'Sprint 1');
    await expect(page.getByTestId(`backlog-section-${planned?.id}`)).toContainText('2 incidencias');

    // Start it.
    await page.getByTestId('start-sprint').getByRole('button').click();
    await expect(page.getByRole('dialog', { name: 'Iniciar sprint' })).toBeVisible();
    await page.getByTestId('sprint-confirm').getByRole('button').click();
    await expect(page.locator('.backlog-section--active')).toContainText('Sprint 1');
    expect((await listSprintsViaApi(owner, project.id)).some((s) => s.status === 'active')).toBe(
      true,
    );

    // Finish one issue through the workflow so the report has something completed.
    const transitions = await owner.get(`issues/${done.id}/transitions`);
    const options = (
      (await transitions.json()) as { data: Array<{ id: string; to: { category: string } }> }
    ).data;
    const finish = options.find((item) => item.to.category === 'done');
    if (finish) {
      const result = await owner.post(`issues/${done.id}/transitions`, {
        data: { transition_id: finish.id },
      });
      expect(result.ok(), 'finishing the issue').toBe(true);
    }
    await page.reload();

    // Complete it: the unfinished issue goes to a new sprint.
    await page
      .locator('.backlog-section--active')
      .getByTestId('complete-sprint')
      .getByRole('button')
      .click();
    const dialog = page.getByTestId('complete-sprint-dialog');
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId('complete-open')).toContainText('1 incidencia sin terminar');
    await page.getByTestId('destination-new').check();
    await page.getByTestId('complete-confirm').getByRole('button').click();

    await expect(page).toHaveURL(/\/sprints\/[^/]+\/report$/);
    await expect(page.getByRole('heading', { name: /Informe de Sprint 1/ })).toBeVisible();
    await expect(page.getByTestId('report-tiles')).toContainText('Comprometidos');
    await expect(page.getByTestId('report-list-incomplete')).toContainText(open.key);
    if (finish) await expect(page.getByTestId('report-list-completed')).toContainText(done.key);

    const sprints = await listSprintsViaApi(owner, project.id);
    expect(sprints.filter((s) => s.status === 'closed')).toHaveLength(1);
    expect(sprints.filter((s) => s.status === 'planned')).toHaveLength(1);
  });

  test('arrastrar una incidencia del backlog a un sprint y editar sus puntos', async ({
    as,
    api,
    data,
  }) => {
    const project = await scrumProject(api, data);
    const owner = await api('owner');
    const issue = await createIssueViaApi(owner, project.id, 'Arrastrada', { story_points: 2 });
    const sprint = await owner.post('sprints', {
      params: { project_id: project.id },
      data: { name: 'Sprint arrastre' },
    });
    expect(sprint.status()).toBe(201);
    const sprintId = ((await sprint.json()) as { data: { id: string } }).data.id;

    const page = await as('owner');
    await page.goto(backlogPath(project.key));
    const source = await row(page, issue.key).boundingBox();
    const target = await page.getByTestId(`backlog-list-${sprintId}`).boundingBox();
    if (!source || !target) throw new Error('rows are not visible');
    await page.mouse.move(source.x + 60, source.y + source.height / 2);
    await page.mouse.down();
    await page.mouse.move(source.x + 80, source.y + 10, { steps: 4 });
    await page.mouse.move(target.x + target.width / 2, target.y + target.height / 2, { steps: 14 });
    await page.mouse.up();
    await expect(
      page.getByTestId(`backlog-section-${sprintId}`).getByTestId(`backlog-row-${issue.key}`),
    ).toBeVisible();

    await page.getByTestId(`points-${issue.key}`).click();
    const input = page.getByTestId(`points-input-${issue.key}`);
    await input.fill('8');
    await input.press('Enter');
    await expect(page.getByTestId(`points-${issue.key}`)).toContainText('8');
    await expect
      .poll(async () => {
        const read = await owner.get(`issues/${issue.id}`);
        return ((await read.json()) as { data: { story_points: number } }).data.story_points;
      })
      .toBe(8);
  });
});

test.describe('diseño del backlog', () => {
  test('el backlog llena el ancho y no deja espacio muerto', async ({ as, api, data, audit }) => {
    const project = await scrumProject(api, data);
    const owner = await api('owner');
    // A planning session: a planned sprint with work in it and a backlog with several issues.
    const ids: string[] = [];
    for (const summary of [
      'Para el diseño',
      'Alta de usuarios',
      'Baja de usuarios',
      'Exportar a CSV',
      'Notificaciones',
      'Panel de métricas',
      'Modo oscuro',
      'Integración con calendario',
    ]) {
      ids.push((await createIssueViaApi(owner, project.id, summary, { story_points: 3 })).id);
    }
    const sprint = await owner.post('sprints', {
      params: { project_id: project.id },
      data: { name: 'Sprint de diseño', goal: 'Comprobar el diseño de la pantalla' },
    });
    const sprintId = ((await sprint.json()) as { data: { id: string } }).data.id;
    const ranked = await owner.put('backlog/rank', {
      data: { issue_ids: ids.slice(0, 3), target: { sprint_id: sprintId } },
    });
    expect(ranked.ok(), 'planning issues into the sprint').toBe(true);
    const page = await as('owner');
    await page.goto(backlogPath(project.key));
    await expect(page.getByText('Para el diseño')).toBeVisible();
    await audit(page, { template: 'fluid', name: 'backlog' });
  });
});
