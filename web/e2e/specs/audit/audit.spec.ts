import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';
import { appPath } from '../../lib/paths.ts';

interface AuditRow {
  readonly action: string;
  readonly summary: string;
  readonly request_id: string;
}

test.describe('auditoría', () => {
  test('el propietario ve los eventos que genera la propia prueba, filtra y abre el detalle', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    // Creating an issue is audited with its key in the summary.
    const project = await data.createProject('owner', orgId, 'audit');
    const issue = await createIssueViaApi(owner, project.id, data.uniqueName('auditada'));
    const key = issue.key;

    // The audit entry of the team that was just created, read through the API.
    let created: AuditRow | undefined;
    await expect
      .poll(async () => {
        const response = await owner.get(`organizations/${orgId}/audit`, { params: { limit: 20 } });
        const rows = ((await response.json()) as { data: AuditRow[] }).data;
        created = rows.find((row) => row.summary.includes(key));
        return created !== undefined;
      })
      .toBe(true);

    const page = await as('owner');
    await page.goto(appPath('/organization/audit'));
    await expect(page.getByRole('heading', { name: 'Auditoría', exact: true })).toBeVisible();
    await expect(page.getByTestId('audit-table')).toBeVisible();
    await expect(page.getByTestId('audit-row').filter({ hasText: key }).first()).toBeVisible();

    // Filter by the exact action: the entry of the new team is still listed.
    await page.getByTestId('audit-action').fill(created?.action ?? '');
    await page.getByTestId('audit-apply').getByRole('button').click();
    await expect(page.getByTestId('audit-row').first()).toBeVisible();
    await expect(page.getByTestId('audit-row').filter({ hasText: key }).first()).toBeVisible();

    // Detail drawer: summary, request id and the changes as text.
    await page
      .getByTestId('audit-row')
      .filter({ hasText: key })
      .first()
      .getByTestId('audit-open')
      .click();
    const drawer = page.getByRole('dialog', { name: 'Detalle del registro' });
    await expect(drawer.getByTestId('audit-summary')).toContainText(key);
    await expect(drawer.getByTestId('audit-request-id')).toHaveText(created?.request_id ?? '');
    await drawer.getByRole('button', { name: 'Cerrar' }).click();
    await expect(drawer).toBeHidden();

    // A period in the future has nothing.
    const tomorrow = new Date(Date.now() + 2 * 86_400_000);
    const iso = `${String(tomorrow.getDate()).padStart(2, '0')}/${String(tomorrow.getMonth() + 1).padStart(2, '0')}/${tomorrow.getFullYear()}`;
    await page.getByLabel('Desde').pressSequentially(iso);
    await page.getByLabel('Desde').press('Tab');
    await page.getByTestId('audit-apply').getByRole('button').click();
    await expect(page.getByTestId('audit-empty')).toBeVisible();
    await page.getByTestId('audit-clear').getByRole('button').click();
    await expect(page.getByTestId('audit-table')).toBeVisible();
  });

  test('exporta el CSV con los filtros activos', async ({ as }) => {
    const page = await as('owner');
    await page.goto(appPath('/organization/audit'));
    await expect(page.getByTestId('audit-table')).toBeVisible();
    const download = page.waitForEvent('download');
    await page.getByTestId('audit-export').getByRole('button').click();
    const file = await download;
    expect(file.suggestedFilename()).toMatch(/^auditoria-\d{4}-\d{2}-\d{2}\.csv$/);
  });

  test('quien no administra la organización no entra', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/organization/audit'));
    await expect(page.getByRole('heading', { name: 'Auditoría', exact: true })).toHaveCount(0);
    await expect(
      page.getByRole('heading', { name: 'No tienes acceso a esta página' }).first(),
    ).toBeVisible();
  });

  test('el administrador de la plataforma ve el registro global', async ({ as }) => {
    const page = await as('admin');
    await page.goto(appPath('/organization/platform-audit'));
    await expect(
      page.getByRole('heading', { name: 'Auditoría de la plataforma' }).first(),
    ).toBeVisible();
    await expect(page.getByTestId('audit-table')).toBeVisible();
  });

  test('la página cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('owner');
    await page.goto(appPath('/organization/audit'));
    await expect(page.getByTestId('audit-table')).toBeVisible();
    await audit(page, { template: 'wide', name: 'audit' });
  });
});
