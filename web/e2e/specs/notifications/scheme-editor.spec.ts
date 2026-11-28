import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';

const ROW = 'scheme-row-issue_created';
const EMAIL_SWITCH = 'scheme-channel-issue_created-email';

test.describe('notificaciones del proyecto', () => {
  test('el propietario personaliza una regla, la guarda, persiste al recargar y la restablece', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'notif');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/notifications`));

    const row = page.getByTestId(ROW);
    await expect(row.getByTestId('scheme-badge-inherited')).toBeVisible();
    await expect(page.getByTestId('scheme-dirty')).toHaveCount(0);

    // Editing a channel makes the row the project's own and shows the unsaved notice.
    await page.getByTestId(EMAIL_SWITCH).click();
    await expect(row.getByTestId('scheme-badge-own')).toBeVisible();
    await expect(page.getByTestId('scheme-dirty')).toBeVisible();
    await page.getByTestId('scheme-save').getByRole('button').click();
    await expect(page.getByTestId('scheme-dirty')).toHaveCount(0);

    // The server keeps only that row as its own and it survives a reload.
    const saved = await owner.get(`projects/${project.id}/notification-scheme`);
    const rules = (
      (await saved.json()) as {
        data: { rules: Array<{ event: string; overridden: boolean; channels: string[] }> };
      }
    ).data.rules;
    expect(rules.filter((rule) => rule.overridden).map((rule) => rule.event)).toEqual([
      'issue_created',
    ]);
    await page.reload();
    await expect(page.getByTestId(ROW).getByTestId('scheme-badge-own')).toBeVisible();
    await expect(page.getByTestId(EMAIL_SWITCH).getByRole('switch')).toBeChecked();

    // Reset asks for confirmation and brings the inherited values back.
    await page.getByTestId('scheme-reset').getByRole('button').click();
    const confirm = page.getByRole('dialog', { name: 'Restablecer las notificaciones' });
    await confirm.getByRole('button', { name: 'Restablecer' }).click();
    await expect(page.getByTestId(ROW).getByTestId('scheme-badge-inherited')).toBeVisible();

    await owner.delete(`projects/${project.id}/notification-scheme`);
  });

  test('cancelar el restablecimiento no cambia nada y descartar deshace las ediciones', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'notif-undo');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/notifications`));

    await page.getByTestId(EMAIL_SWITCH).click();
    await expect(page.getByTestId('scheme-dirty')).toBeVisible();
    await page.getByTestId('scheme-discard').getByRole('button').click();
    await expect(page.getByTestId('scheme-dirty')).toHaveCount(0);
    await expect(page.getByTestId(ROW).getByTestId('scheme-badge-inherited')).toBeVisible();
  });

  test('quien no puede editar el proyecto no ve el esquema', async ({ as, api, data }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'notif-ro');
    const page = await as('viewer');
    await page.goto(appPath(`/projects/${project.key}/settings/notifications`));
    await expect(page.getByTestId('scheme-save')).toHaveCount(0);
  });

  test('la página cumple la auditoría de layout', async ({ as, api, data, audit }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'notif-audit');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/notifications`));
    await expect(page.getByTestId(ROW)).toBeVisible();
    await audit(page, { template: 'wide', name: 'project-notifications' });
  });
});

test.describe('notificaciones por defecto de la organización', () => {
  test('el administrador personaliza el esquema, lo ve guardado y lo restablece', async ({
    as,
    api,
  }) => {
    const admin = await api('orgadmin');
    const orgId = await orgIdBySlug(admin, 'acme');
    const page = await as('orgadmin');
    try {
      await page.goto(appPath('/organization/notifications'));
      await expect(
        page.getByRole('heading', { name: 'Notificaciones', exact: true }),
      ).toBeVisible();

      await page.getByTestId(EMAIL_SWITCH).click();
      await page.getByTestId('scheme-save').getByRole('button').click();
      await expect(page.getByTestId('scheme-dirty')).toHaveCount(0);

      const saved = await admin.get(`organizations/${orgId}/notification-scheme`);
      const body = (await saved.json()) as { data: { is_default: boolean } };
      expect(body.data.is_default).toBe(false);

      await page.reload();
      await expect(page.getByTestId(ROW).getByTestId('scheme-badge-own')).toBeVisible();
      await page.getByTestId('scheme-reset').getByRole('button').click();
      await page
        .getByRole('dialog', { name: 'Restablecer las notificaciones' })
        .getByRole('button', { name: 'Restablecer' })
        .click();
      await expect(page.getByTestId(ROW).getByTestId('scheme-badge-inherited')).toBeVisible();
    } finally {
      await admin.delete(`organizations/${orgId}/notification-scheme`);
    }
  });

  test('la página cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('orgadmin');
    await page.goto(appPath('/organization/notifications'));
    await expect(page.getByTestId(ROW)).toBeVisible();
    await audit(page, { template: 'wide', name: 'org-notifications' });
  });
});
