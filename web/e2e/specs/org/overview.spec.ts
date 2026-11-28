import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

test.describe('resumen de la organización', () => {
  test('el administrador aterriza en el resumen con cifras, actividad y accesos', async ({
    as,
  }) => {
    const page = await as('orgadmin');
    await page.goto(appPath('/organization'));
    await expect(page).toHaveURL(/\/organization\/overview$/);
    await expect(page.getByRole('heading', { name: 'Acme', exact: true })).toBeVisible();

    const tiles = page.getByTestId('overview-tiles');
    await expect(tiles).toContainText('Miembros');
    await expect(tiles).toContainText('Proyectos');
    await expect(tiles).toContainText('Invitaciones pendientes');
    await expect(
      page.getByTestId('overview-activity').or(page.getByTestId('overview-activity-empty')),
    ).toBeVisible();

    const shortcuts = page.getByTestId('overview-shortcuts');
    await expect(shortcuts.getByRole('link', { name: /Notificaciones/ })).toBeVisible();
    // Platform-only pages are not offered to an organization administrator.
    await expect(shortcuts.getByRole('link', { name: /Cola de correo/ })).toHaveCount(0);

    await shortcuts.getByRole('link', { name: /Miembros/ }).click();
    await expect(page).toHaveURL(/\/organization\/members$/);
  });

  test('el administrador de la plataforma ve además los accesos de la plataforma', async ({
    as,
  }) => {
    const page = await as('admin');
    await page.goto(appPath('/organization/overview'));
    const shortcuts = page.getByTestId('overview-shortcuts');
    await expect(shortcuts.getByRole('link', { name: /Cola de correo/ })).toBeVisible();
    await expect(shortcuts.getByRole('link', { name: /Trabajos programados/ })).toBeVisible();
  });

  test('un miembro sin permisos no entra al resumen', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/organization/overview'));
    await expect(page.getByTestId('overview-tiles')).toHaveCount(0);
  });

  test('la página cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('orgadmin');
    await page.goto(appPath('/organization/overview'));
    await expect(page.getByTestId('overview-tiles')).toBeVisible();
    await audit(page, { template: 'wide', name: 'org-overview' });
  });
});
