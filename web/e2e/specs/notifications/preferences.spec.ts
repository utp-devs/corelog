import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

test.describe('preferencias de notificación de la persona', () => {
  test('guarda el modo de correo y la opción de acciones propias, y persisten al recargar', async ({
    as,
    api,
  }) => {
    const member = await api('member');
    const page = await as('member');
    try {
      await page.goto(appPath('/profile/notifications'));
      await expect(page.getByTestId('notifications-digest-note')).toContainText('08:00 UTC');

      await page.getByRole('combobox', { name: 'Correo' }).click();
      await page.getByRole('option', { name: 'Resumen diario', exact: true }).click();
      await page
        .locator('p-toggleswitch:has(input[aria-label="Notificarme de mis propias acciones"])')
        .click();
      await expect(page.getByRole('status').filter({ hasText: 'propias acciones' })).toBeVisible();

      const saved = await member.get('notifications/preferences');
      expect(((await saved.json()) as { data: unknown }).data).toMatchObject({
        email: 'daily',
        notify_own_actions: true,
      });

      await page.reload();
      await expect(page.getByRole('combobox', { name: 'Correo' })).toContainText('Resumen diario');
      await expect(
        page.getByRole('switch', { name: 'Notificarme de mis propias acciones' }),
      ).toBeChecked();
    } finally {
      await member.put('notifications/preferences', {
        data: { in_app: true, email: 'immediate', notify_own_actions: false },
      });
    }
  });

  test('la sección cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('member');
    await page.goto(appPath('/profile/notifications'));
    await expect(page.getByTestId('notifications-digest-note')).toBeVisible();
    await audit(page, { template: 'content', name: 'notification-preferences' });
  });
});
