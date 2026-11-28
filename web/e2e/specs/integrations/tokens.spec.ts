import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

interface ApiTokenRow {
  readonly id: string;
  readonly name: string;
  readonly revoked_at: string | null;
}

test.describe('tokens de acceso personales', () => {
  test('crea un token, lo muestra una sola vez y lo revoca', async ({ as, api, data }) => {
    const owner = await api('owner');
    const name = data.uniqueName('token');
    const page = await as('owner');
    await page.goto(appPath('/profile/tokens'));
    await expect(
      page.getByRole('heading', { name: 'Tokens de acceso', exact: true }),
    ).toBeVisible();

    await page.getByTestId('token-new').getByRole('button').click();
    await page.getByTestId('token-name').fill(name);
    await page.getByTestId('token-scope-write').check();
    await page.getByTestId('token-create').getByRole('button').click();

    // The plain token is shown once, read-only, and cannot be dismissed before confirming the copy.
    const reveal = page.getByTestId('secret-reveal');
    await expect(reveal).toBeVisible();
    await expect(reveal).toContainText('No volverás a ver este token');
    const field = page.getByTestId('secret-reveal-value');
    await expect(field).toHaveAttribute('readonly', '');
    const secret = await field.inputValue();
    expect(secret).toMatch(/^cl_pat_/);
    await expect(page.getByTestId('secret-reveal-close').getByRole('button')).toBeDisabled();

    await page.getByTestId('secret-reveal-ack').check();
    await page.getByTestId('secret-reveal-close').getByRole('button').click();
    await expect(reveal).toBeHidden();

    // The list shows the prefix and the scope, never the token itself.
    const item = page.getByTestId('token-item').filter({ hasText: name });
    await expect(item).toBeVisible();
    await expect(item).toContainText('Lectura y escritura');
    await expect(item).toContainText('Activo');
    expect(await page.content()).not.toContain(secret);

    // After a reload the secret is gone for good.
    await page.reload();
    await expect(page.getByTestId('token-item').filter({ hasText: name })).toBeVisible();
    expect(await page.content()).not.toContain(secret);

    // Revoke with confirmation.
    await page
      .getByTestId('token-item')
      .filter({ hasText: name })
      .getByTestId('token-revoke')
      .getByRole('button')
      .click();
    await page.getByRole('button', { name: 'Revocar token' }).click();
    await expect(page.getByTestId('token-item').filter({ hasText: name })).toHaveCount(0);

    // Revoked tokens are hidden from the list, also after a reload.
    await page.reload();
    await expect(page.getByTestId('token-new')).toBeVisible();
    await expect(page.getByTestId('token-item').filter({ hasText: name })).toHaveCount(0);
    const visible = await owner.get('me/api-tokens');
    const visibleRows = ((await visible.json()) as { data: ApiTokenRow[] }).data;
    expect(visibleRows.find((row) => row.name === name)).toBeUndefined();

    // The secondary toggle brings them back, dimmed and with their revocation date.
    await page.locator('label.api-tokens__toggle').click();
    const revoked = page.getByTestId('token-item').filter({ hasText: name });
    await expect(revoked).toContainText('Revocado');
    await expect(revoked.getByTestId('token-revoked-at')).toBeVisible();

    const all = await owner.get('me/api-tokens?include_revoked=true');
    const rows = ((await all.json()) as { data: ApiTokenRow[] }).data;
    expect(rows.find((row) => row.name === name)?.revoked_at).toBeTruthy();
  });

  test('pide un nombre antes de crear el token', async ({ as, data }) => {
    const page = await as('owner');
    await page.goto(appPath('/profile/tokens'));
    await page.getByTestId('token-new').getByRole('button').click();
    await expect(page.getByTestId('token-create').getByRole('button')).toBeDisabled();
    await page.getByTestId('token-name').fill(data.uniqueName('x'));
    await expect(page.getByTestId('token-create').getByRole('button')).toBeEnabled();
  });

  test('la página cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('owner');
    await page.goto(appPath('/profile/tokens'));
    await expect(page.getByTestId('token-new')).toBeVisible();
    await audit(page, { template: 'reading', name: 'profile-tokens' });
  });
});
