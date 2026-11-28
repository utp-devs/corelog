import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';

interface RuleRow {
  readonly id: string;
  readonly name: string;
  readonly run_count: number;
}
interface RunRow {
  readonly status: string;
  readonly issue_id: string | null;
}
interface CommentRow {
  readonly body?: string;
  readonly content?: string;
}

test.describe('automatización', () => {
  test('crea una regla, la valida, la activa y la ve actuar sobre una incidencia nueva', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'auto');
    const ruleName = data.uniqueName('regla');
    const page = await as('owner');

    await page.goto(appPath(`/projects/${project.key}/settings/automation`));
    await expect(page.getByTestId('rules-empty')).toBeVisible();

    // Builder: the default trigger is "se crea una incidencia"; the action is a comment with a smart value.
    await page.getByTestId('rule-new').click();
    await page.getByTestId('rule-name').fill(ruleName);
    await page.getByTestId('rule-action-text').fill('Bienvenida a ');
    await page.getByTestId('rule-smart-issue.key').click();
    await expect(page.getByTestId('rule-action-text')).toHaveValue('Bienvenida a {{issue.key}}');
    await expect(page.getByTestId('rule-summary')).toContainText(
      'Cuando se cree una incidencia, entonces añadir el comentario «Bienvenida a {{issue.key}}»',
    );

    // Actions can be reordered with the keyboard-accessible buttons.
    await page.getByTestId('rule-add-action').click();
    await page.getByTestId('rule-action-type-1').selectOption('add_label');
    await page.getByTestId('rule-action-up').nth(1).click();
    await expect(page.getByTestId('rule-action-type-0')).toHaveValue('add_label');
    await page.getByTestId('rule-action-remove').first().click();
    await expect(page.getByTestId('rule-action-type-0')).toHaveValue('add_comment');

    await page.getByTestId('rule-validate').getByRole('button').click();
    await expect(page.getByTestId('rule-validation')).toContainText('La regla es válida');

    await page.getByTestId('rule-save').getByRole('button').click();
    const item = page.getByTestId('rule-item').filter({ hasText: ruleName });
    await expect(item).toBeVisible();
    await expect(item).toContainText('Cuando se cree una incidencia');

    // Turn it off and on again with the switch.
    await item.getByTestId('rule-enabled').getByRole('switch').click();
    await expect(item).toContainText('Desactivada');
    await item.getByTestId('rule-enabled').getByRole('switch').click();
    await expect(item).toContainText('Activa');

    const listed = await owner.get(`projects/${project.id}/automation-rules`);
    const rules = ((await listed.json()) as { data: RuleRow[] }).data;
    const rule = rules.find((r) => r.name === ruleName);
    expect(rule).toBeTruthy();

    // A new issue fires the rule: the run succeeds and the comment appears.
    const issue = await createIssueViaApi(owner, project.id, data.uniqueName('disparo'));
    await expect
      .poll(
        async () => {
          const response = await owner.get(`automation-rules/${rule?.id}/runs`);
          const runs = ((await response.json()) as { data: RunRow[] }).data;
          return runs.some((run) => run.status === 'success' && run.issue_id === issue.id);
        },
        { timeout: 45_000 },
      )
      .toBe(true);
    const comments = await owner.get(`issues/${issue.id}/comments`);
    const rows = ((await comments.json()) as { data: CommentRow[] }).data;
    expect(
      rows.some((c) => (c.body ?? c.content ?? '').includes(`Bienvenida a ${issue.key}`)),
    ).toBe(true);

    // The history drawer shows the run.
    await page.reload();
    const again = page.getByTestId('rule-item').filter({ hasText: ruleName });
    await expect(again.getByTestId('rule-run-count')).not.toHaveText('0');
    await again.getByTestId('rule-runs').getByRole('button').click();
    const drawer = page.getByRole('dialog', { name: new RegExp(`Historial de ${ruleName}`) });
    await expect(drawer.getByTestId('run-item').first()).toContainText('Correcta');
    await drawer.getByRole('button', { name: 'Cerrar', exact: true }).last().click();

    // Delete the rule after confirming.
    await again.getByTestId('rule-delete').getByRole('button').click();
    await page.getByRole('button', { name: 'Eliminar regla' }).click();
    await expect(page.getByTestId('rule-item').filter({ hasText: ruleName })).toHaveCount(0);
  });

  test('muestra los errores de una regla incompleta antes de guardar', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'auto');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/automation/new`));
    await page.getByTestId('rule-save').getByRole('button').click();
    await expect(page.getByRole('alert').filter({ hasText: 'Escribe un nombre' })).toBeVisible();
    await expect(
      page.getByRole('alert').filter({ hasText: 'El texto es obligatorio' }),
    ).toBeVisible();
  });

  test('quien no gestiona automatizaciones no entra', async ({ as, api, data }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'auto');
    const page = await as('viewer');
    await page.goto(appPath(`/projects/${project.key}/settings/automation`));
    await expect(page.getByTestId('rule-new')).toHaveCount(0);
  });

  test('las páginas cumplen la auditoría de layout', async ({ as, api, data, audit }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'auto');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/automation`));
    await expect(page.getByTestId('rule-new')).toBeVisible();
    await audit(page, { template: 'wide', name: 'automation-list' });
    await page.getByTestId('rule-new').click();
    await expect(page.getByTestId('rule-name')).toBeVisible();
    await audit(page, { template: 'wide', name: 'automation-builder' });
  });
});
