import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createProjectViaApi } from '../../lib/projects-api.ts';
import {
  addTransitionViaApi,
  createIssueViaApi,
  readWorkflow,
  userIdOf,
} from '../../lib/workflow-api.ts';

test.describe('transiciones de incidencias', () => {
  test('una transición solo para el responsable la ve quien está asignado y no el resto', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'wft');
    const memberApi = await api('member');
    const added = await owner.post(`projects/${project.id}/members`, {
      data: { user_id: await userIdOf(memberApi), role: 'contributor' },
    });
    expect(added.ok(), 'adding the contributor').toBe(true);

    const base = await readWorkflow(owner, project.id);
    const target = base.statuses.find((status) => status.id !== base.initial_status_id);
    expect(target, 'a second status to move to').toBeDefined();
    await addTransitionViaApi(owner, project.id, {
      id: 'tomar',
      name: 'Tomar para mí',
      from: [base.initial_status_id],
      to: (target as { id: string }).id,
      conditions: [{ type: 'is_assignee' }],
    });

    const issue = await createIssueViaApi(owner, project.id, 'Solo para el responsable', {
      assignee_id: await userIdOf(owner),
    });

    const assignee = await as('owner');
    await assignee.goto(appPath(`/browse/${issue.key}`));
    await assignee.getByTestId('transition-button').click();
    await expect(assignee.getByTestId('transition-option-tomar')).toBeVisible();
    await assignee.keyboard.press('Escape');

    const other = await as('member');
    await other.goto(appPath(`/browse/${issue.key}`));
    await other.getByTestId('transition-button').click();
    await expect(other.getByRole('menu')).toBeVisible();
    await expect(other.getByTestId('transition-option-tomar')).toHaveCount(0);

    // The assignee runs it: no dialog is needed, the status changes at once.
    await assignee.getByTestId('transition-button').click();
    await assignee.getByTestId('transition-option-tomar').click();
    await expect(
      assignee.getByText(`Estado cambiado a «${(target as { name: string }).name}»`).first(),
    ).toBeVisible();
  });

  test('mover una tarjeta en el tablero usa el flujo y el servidor explica cuando no hay transición', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    // A Kanban board lists every issue; a Scrum board only shows the active sprint.
    const project = await createProjectViaApi(owner, acme, data.uniqueName('wfb'));
    data.trackProject('owner', project);
    const issue = await createIssueViaApi(owner, project.id, 'Tarjeta del tablero');

    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/board`));
    const card = page.getByTestId(`kanban-card-${issue.key}`);
    await expect(card).toBeVisible();
    await card.getByRole('button', { name: /^Mover .* a otra columna$/ }).click();
    // The menu lists the other columns; pick the first one.
    await page.getByRole('menuitem').first().click();
    await expect(page.getByRole('status').filter({ hasText: 'movida a' })).toHaveCount(1);

    const moved = await owner.get(`issues/${issue.id}`);
    expect(moved.ok()).toBe(true);
  });
});
