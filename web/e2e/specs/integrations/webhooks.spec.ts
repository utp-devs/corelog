import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';

test.describe('webhooks de la organización', () => {
  test('crea un webhook, muestra el secreto una vez, prueba el envío y lo elimina', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const orgId = await orgIdBySlug(owner, 'acme');
    const name = data.uniqueName('webhook');
    const page = await as('owner');
    await page.goto(appPath('/organization/webhooks'));
    await expect(page.getByRole('heading', { name: 'Webhooks', exact: true })).toBeVisible();

    await page.getByTestId('webhook-new').getByRole('button').click();
    await page.getByTestId('webhook-name').fill(name);

    // Live validation of the address: private hosts are refused, public ones accepted.
    await page.getByTestId('webhook-url').fill('http://localhost:9000/x');
    await expect(page.getByTestId('webhook-url-error')).toContainText('privadas');
    await expect(page.getByTestId('webhook-save').getByRole('button')).toBeDisabled();
    await page.getByTestId('webhook-url').fill('https://example.com/hook');
    await expect(page.getByTestId('webhook-url-error')).toHaveCount(0);

    // Events: all by default, narrow it to two.
    await page.getByTestId('webhook-event-all').uncheck();
    await expect(page.getByTestId('webhook-save').getByRole('button')).toBeDisabled();
    await page.getByTestId('webhook-event-issue.created').check();
    await page.getByTestId('webhook-event-comment.created').check();
    await page.getByTestId('webhook-save').getByRole('button').click();

    const reveal = page.getByTestId('secret-reveal');
    await expect(reveal).toBeVisible();
    const secret = await page.getByTestId('secret-reveal-value').inputValue();
    expect(secret.length).toBeGreaterThan(10);
    await page.getByTestId('secret-reveal-ack').check();
    await page.getByTestId('secret-reveal-close').getByRole('button').click();
    await expect(reveal).toBeHidden();

    const item = page.getByTestId('webhook-item').filter({ hasText: name });
    await expect(item).toBeVisible();
    await expect(item).toContainText('example.com');
    await expect(item).toContainText('Se crea una incidencia');
    expect(await page.content()).not.toContain(secret);

    // Test ping: no receiver is reachable from the stack, so only the result states are asserted.
    await item.getByTestId('webhook-test').getByRole('button').click();
    await expect(item.getByTestId('webhook-ping-result')).toBeVisible({ timeout: 30_000 });

    // Deliveries drawer lists the ping and filters by status.
    await item.getByTestId('webhook-deliveries').getByRole('button').click();
    const drawer = page.getByRole('dialog', { name: new RegExp(`Entregas de ${name}`) });
    await expect(drawer).toBeVisible();
    await expect(drawer.getByTestId('delivery-item').first()).toBeVisible();
    await drawer.getByTestId('delivery-payload-toggle').first().click();
    await expect(drawer.getByLabel('Contenido enviado').first()).toContainText('"event"');
    await drawer.getByTestId('delivery-filter-dead').click();
    await drawer.getByRole('button', { name: 'Cerrar', exact: true }).last().click();
    await expect(drawer).toBeHidden();

    // Switch off and edit.
    await item.getByTestId('webhook-enabled').getByRole('switch').click();
    await expect(item).toContainText('Desactivado');
    const listed = await owner.get(`organizations/${orgId}/webhooks`);
    const hooks = ((await listed.json()) as { data: Array<{ name: string; enabled: boolean }> })
      .data;
    expect(hooks.find((h) => h.name === name)?.enabled).toBe(false);

    // Delete with the typed confirmation.
    await item.getByTestId('webhook-delete').getByRole('button').click();
    const confirm = page.getByRole('dialog', { name: 'Eliminar webhook' });
    const button = confirm.getByRole('button', { name: 'Eliminar webhook' });
    await expect(button).toBeDisabled();
    await confirm.getByRole('textbox').fill(name);
    await button.click();
    await expect(page.getByTestId('webhook-item').filter({ hasText: name })).toHaveCount(0);
  });

  test('quien no administra la organización no entra', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/organization/webhooks'));
    await expect(page.getByRole('heading', { name: 'Webhooks', exact: true })).toHaveCount(0);
  });

  test('la página cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('owner');
    await page.goto(appPath('/organization/webhooks'));
    await expect(page.getByTestId('webhook-new')).toBeVisible();
    await audit(page, { template: 'wide', name: 'webhooks' });
  });
});
