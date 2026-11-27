import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { pickSelectOption } from '../../lib/ui.ts';
import { createIssueViaApi, readWorkflow, userIdOf } from '../../lib/workflow-api.ts';

function workflowPath(projectKey: string): string {
  return appPath(`/projects/${projectKey}/settings/workflow`);
}

test.describe('editor del flujo de trabajo', () => {
  test('el líder añade un estado y una transición con comentario obligatorio, valida y guarda; luego ejecuta la transición', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'wf');
    const statusName = data.uniqueName('bloq').slice(0, 24);
    const issue = await createIssueViaApi(owner, project.id, 'Incidencia del flujo');

    const page = await as('owner');
    await page.goto(workflowPath(project.key));
    await expect(page.getByRole('heading', { name: 'Flujo de trabajo', level: 2 })).toBeVisible();
    await expect(page.getByTestId('workflow-status').first()).toBeVisible();
    await expect(page.getByTestId('workflow-dirty')).toHaveCount(0);

    // A new status.
    await page.getByTestId('workflow-add-status').click();
    await page.getByTestId('workflow-new-status-name').fill(statusName);
    await page.getByTestId('workflow-add-status-confirm').click();
    await expect(
      page.locator(`[data-testid="workflow-status"][data-name="${statusName}"]`),
    ).toBeVisible();
    await expect(page.getByTestId('workflow-dirty')).toBeVisible();

    // A transition into it that asks for a comment.
    await page.getByTestId('workflow-add-transition').click();
    const panel = page.getByTestId('workflow-transition-panel');
    await expect(panel).toBeVisible();
    await page.getByTestId('workflow-transition-name').fill('Bloquear');
    await pickSelectOption(page, 'Hasta', statusName);
    await pickSelectOption(page, 'Añadir validaciones', 'Exigir un comentario');
    await expect(panel.getByTestId('workflow-rule')).toHaveCount(1);

    // Validate, then save.
    await page.getByTestId('workflow-validate').click();
    await expect(page.getByTestId('workflow-valid')).toBeVisible();
    await page.getByTestId('workflow-save').click();
    await expect(page.getByText('Flujo de trabajo guardado').first()).toBeVisible();
    await expect(page.getByTestId('workflow-dirty')).toHaveCount(0);

    const saved = await readWorkflow(owner, project.id);
    const created = saved.transitions.find((item) => item.name === 'Bloquear');
    expect(created?.validators).toMatchObject([{ type: 'comment_required' }]);

    // The issue can now be blocked, but only with a comment.
    await page.goto(appPath(`/browse/${issue.key}`));
    await page.getByTestId('transition-button').click();
    await page.getByTestId(`transition-option-${created?.id}`).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByTestId('transition-confirm').click();
    await expect(dialog).toContainText('Escribe un comentario.');
    await dialog.getByTestId('transition-comment').fill('Esperando a otro equipo');
    await dialog.getByTestId('transition-confirm').click();
    await expect(page.getByText(`Estado cambiado a «${statusName}»`).first()).toBeVisible();
    await expect(page.getByTestId('transition-button')).toContainText(statusName);
  });

  test('el diagrama dibuja los estados y las transiciones y la tabla es su alternativa textual', async ({
    as,
    api,
    data,
    audit,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'wfd');
    const workflow = await readWorkflow(owner, project.id);

    const page = await as('owner');
    await page.goto(workflowPath(project.key));
    await expect(page.getByTestId('workflow-transitions')).toBeVisible();
    await audit(page, { template: 'wide', name: 'workflow-editor-table' });

    await page.getByTestId('workflow-tab-diagram').click();
    const diagram = page.getByTestId('workflow-diagram');
    await expect(diagram).toBeVisible();
    await expect(
      page.locator('.workflow-diagram__node:not(.workflow-diagram__node--any)'),
    ).toHaveCount(workflow.statuses.length);
    await expect(page.locator('.workflow-diagram__edge').first()).toBeVisible();
    await page.getByRole('button', { name: 'Acercar' }).click();
    await expect(diagram).toContainText('120 %');
    await page.getByRole('button', { name: 'Ajustar a la vista' }).click();
    await expect(diagram).toContainText('100 %');
    await audit(page, { template: 'wide', name: 'workflow-editor-diagram' });

    // Choosing an edge opens its panel.
    await page.locator('.workflow-diagram__edge').first().click();
    await expect(page.getByTestId('workflow-transition-panel')).toBeVisible();
  });

  test('quien no gestiona el flujo lo ve en solo lectura y un observador no ve el botón de transición', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'wfr');
    const issue = await createIssueViaApi(owner, project.id, 'Incidencia de solo lectura');
    for (const [role, projectRole] of [
      ['member', 'contributor'],
      ['viewer', 'viewer'],
    ] as const) {
      const added = await owner.post(`projects/${project.id}/members`, {
        data: { user_id: await userIdOf(await api(role)), role: projectRole },
      });
      expect(added.ok(), `adding ${role}`).toBe(true);
    }

    const member = await as('member');
    await member.goto(workflowPath(project.key));
    await expect(member.getByTestId('workflow-readonly')).toBeVisible();
    await expect(member.getByTestId('workflow-add-status')).toHaveCount(0);
    await expect(member.getByTestId('workflow-add-transition')).toHaveCount(0);
    await expect(member.getByTestId('workflow-templates')).toHaveCount(0);

    // A contributor can transition; an observer cannot.
    await member.goto(appPath(`/browse/${issue.key}`));
    await expect(member.getByTestId('transition-button')).toBeVisible();

    const viewer = await as('viewer');
    await viewer.goto(appPath(`/browse/${issue.key}`));
    await expect(viewer.getByRole('heading', { level: 1 })).toContainText(
      'Incidencia de solo lectura',
    );
    await expect(viewer.getByTestId('transition-button')).toHaveCount(0);
  });
});
