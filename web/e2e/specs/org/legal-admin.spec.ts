import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

test.describe('documentos legales (administración de la plataforma)', () => {
  test('el administrador ve los documentos vigentes y la auditoría de consentimientos', async ({
    as,
  }) => {
    const page = await as('admin');
    await page.goto(appPath('/organization/legal'));
    await expect(page.getByRole('heading', { name: 'Documentos legales' }).first()).toBeVisible();
    await expect(page.getByText('Política de privacidad', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Términos de uso', { exact: true }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Auditoría de consentimientos' })).toBeVisible();
  });

  test('el publicador tiene vista previa y pide confirmación; cancelar no publica nada', async ({
    as,
    api,
  }) => {
    const admin = await api('admin');
    const before = (await (await admin.get('admin/legal/documents')).json()) as { data: unknown[] };
    const page = await as('admin');
    await page.goto(appPath('/organization/legal'));
    await page.getByRole('button', { name: 'Publicar nueva versión' }).first().click();

    const dialog = page.getByRole('dialog', { name: 'Publicar nueva versión' });
    await expect(dialog).toBeVisible();
    await dialog
      .getByRole('textbox', { name: 'Texto (Markdown)' })
      .fill('## Cláusula\n\nTexto **de prueba**');
    await dialog.getByText('Vista previa', { exact: true }).click();
    await expect(dialog.getByTestId('legal-preview').locator('h2')).toHaveText('Cláusula');
    await expect(dialog.getByTestId('legal-preview').locator('strong')).toHaveText('de prueba');

    await dialog.getByRole('button', { name: 'Publicar', exact: true }).click();
    const confirm = page.getByRole('dialog', { name: '¿Publicar esta versión?' });
    await expect(confirm).toContainText(/Vas a publicar la versión \d+/);
    await confirm.getByRole('button', { name: 'Cancelar' }).click();
    await expect(confirm).toBeHidden();
    await dialog.getByRole('button', { name: 'Cancelar' }).click();
    await expect(dialog).toBeHidden();

    const after = (await (await admin.get('admin/legal/documents')).json()) as { data: unknown[] };
    expect(after.data.length).toBe(before.data.length);
  });

  test('un miembro sin permisos no entra a la página', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/organization/legal'));
    await expect(page.getByRole('heading', { name: 'Documentos legales' })).toHaveCount(0);
  });

  test('la página cumple la auditoría de layout', async ({ as, audit }) => {
    const page = await as('admin');
    await page.goto(appPath('/organization/legal'));
    await expect(page.getByText('Documentos publicados')).toBeVisible();
    await expect(page.getByText('Política de privacidad', { exact: true }).first()).toBeVisible();
    await audit(page, { template: 'wide', name: 'legal-admin' });
  });
});
