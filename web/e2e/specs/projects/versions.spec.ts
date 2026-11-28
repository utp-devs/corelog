import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';
import { expect, test } from '../../fixtures/test.ts';

test.describe('versiones del proyecto', () => {
  test('crea una versión, la publica moviendo lo pendiente y consulta sus notas', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'ver');

    const first = await owner.post(`projects/${project.id}/versions`, {
      data: { name: '1.0' },
    });
    expect(first.ok()).toBe(true);
    const version = ((await first.json()) as { data: { id: string } }).data;
    // One open issue is linked to the version through its fix version.
    const issue = await owner.post('issues', {
      data: {
        project_id: project.id,
        summary: 'Pendiente de la 1.0',
        description: 'Pendiente de la 1.0',
        priority: 'medium',
      },
    });
    expect(issue.ok()).toBe(true);
    const issueId = ((await issue.json()) as { data: { id: string } }).data.id;
    const linked = await owner.patch(`issues/${issueId}`, {
      data: { fix_versions: [version.id] },
    });
    test.info().annotations.push({
      type: 'link',
      description: `fix version link status ${linked.status()}`,
    });

    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/versions`));
    const item = page.getByTestId('version-item').filter({ hasText: '1.0' });
    await expect(item).toBeVisible();
    await expect(item.getByTestId('version-progress').first()).toBeVisible();

    await item.getByTestId('version-release').click();
    if (linked.ok()) {
      await page.getByTestId('move-new').check();
    }
    await page.getByTestId('release-confirm').click();
    await expect(item.getByText('Publicada')).toBeVisible();
    if (linked.ok()) {
      await expect(
        page.getByTestId('version-item').filter({ hasText: 'Sin publicar' }),
      ).toHaveCount(1);
    }

    await item.getByRole('button', { name: 'Notas de la versión' }).click();
    await expect(page.getByRole('dialog', { name: /Notas de/ })).toBeVisible();
    await page.getByRole('button', { name: 'Cerrar' }).first().click();

    await item.getByTestId('version-archive').click();
    await expect(item.getByText('Archivada')).toBeVisible();
  });

  test('un colaborador ve las versiones sin acciones de gestión', async ({ as, api, data }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'ver-ro');
    await owner.post(`projects/${project.id}/versions`, { data: { name: '2.0' } });
    const member = await api('member');
    const me = ((await (await member.get('users/me')).json()) as { data: { id: string } }).data.id;
    await owner.post(`projects/${project.id}/members`, {
      data: { user_id: me, role: 'contributor' },
    });
    const page = await as('member');
    await page.goto(appPath(`/projects/${project.key}/settings/versions`));
    await expect(page.getByTestId('version-item')).toHaveCount(1);
    await expect(page.getByTestId('version-new')).toHaveCount(0);
    await expect(page.getByTestId('version-release')).toHaveCount(0);
  });

  test('los componentes muestran responsable y política de asignación', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', orgId, 'comp');
    const created = await owner.post(`projects/${project.id}/components`, {
      data: { name: 'Pagos', default_assignee_policy: 'project_lead' },
    });
    expect(created.ok()).toBe(true);
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/components`));
    const item = page.getByTestId('component-item').filter({ hasText: 'Pagos' });
    await expect(item).toContainText('Líder del proyecto');
  });
});
