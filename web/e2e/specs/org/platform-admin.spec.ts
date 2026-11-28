import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

test.describe('administración de la plataforma: correo y trabajos', () => {
  test('el administrador ve la cola de correo, filtra por estado y activa la actualización', async ({
    as,
  }) => {
    const page = await as('admin');
    await page.goto(appPath('/organization/email-outbox'));
    await expect(page.getByRole('heading', { name: 'Cola de correo', exact: true })).toBeVisible();
    await expect(page.getByRole('switch', { name: 'Actualizar automáticamente' })).toBeChecked();
    await expect(page.getByTestId('outbox-auto')).toContainText('Actualizar cada 10 s');

    await page.getByTestId('outbox-filter').getByText('Fallidos', { exact: true }).click();
    await expect(
      page.getByTestId('outbox-filter').locator('[aria-pressed="true"], [aria-checked="true"]'),
    ).toContainText('Fallidos');
    await page.getByTestId('outbox-refresh').getByRole('button').click();
    await expect(page.getByRole('heading', { name: 'Cola de correo', exact: true })).toBeVisible();
  });

  test('el administrador ve los trabajos programados con su última ejecución', async ({ as }) => {
    const page = await as('admin');
    await page.goto(appPath('/organization/jobs'));
    await expect(
      page.getByRole('heading', { name: 'Trabajos programados', exact: true }),
    ).toBeVisible();
    await expect(page.getByText('email-outbox', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Correcta').first()).toBeVisible();
  });

  test('quien no administra la plataforma ve la página de acceso denegado', async ({ as }) => {
    const page = await as('owner');
    for (const path of ['/organization/email-outbox', '/organization/jobs']) {
      await page.goto(appPath(path));
      await expect(page).toHaveURL(/\/403$/);
      await expect(page.getByRole('heading', { name: 'Cola de correo' })).toHaveCount(0);
    }
  });

  test('las páginas cumplen la auditoría de layout', async ({ as, audit }) => {
    const page = await as('admin');
    await page.goto(appPath('/organization/email-outbox'));
    await expect(page.getByRole('heading', { name: 'Cola de correo', exact: true })).toBeVisible();
    await audit(page, { template: 'wide', name: 'email-outbox' });
    await page.goto(appPath('/organization/jobs'));
    await expect(
      page.getByRole('heading', { name: 'Trabajos programados', exact: true }),
    ).toBeVisible();
    await audit(page, { template: 'wide', name: 'scheduled-jobs' });
  });
});
