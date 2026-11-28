import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';

const BANNER = 'verify-email-banner';

/** The fixture hides the reminder in every page; this brings it back for the current session. */
async function showBanner(page: Page): Promise<void> {
  await page.goto(appPath('/projects'));
  await page.evaluate(() => {
    window.localStorage.setItem('e2e.verify-banner', '1');
    window.sessionStorage.removeItem('corelog.verify-email-banner.dismissed');
  });
  await page.reload();
}

test.describe('aviso de verificación del correo', () => {
  test('aparece con el correo sin verificar, reenvía el enlace y confirma', async ({
    as,
    audit,
  }) => {
    const page = await as('member');
    await page.route('**/auth/email/verify/request', (route) =>
      route.fulfill({ status: 202, json: { data: { message: 'ok' } } }),
    );
    await showBanner(page);
    const banner = page.getByTestId(BANNER);
    await expect(banner).toContainText('member@corelog.test');
    await banner.getByRole('button', { name: 'Reenviar correo' }).click();
    await expect(page.getByTestId('verify-email-feedback')).toContainText('Te enviamos un correo');
    await audit(page, { template: 'wide', name: 'verify-banner' });
  });

  test('explica el límite de envíos y se puede cerrar durante la sesión', async ({ as }) => {
    const page = await as('member');
    await page.route('**/auth/email/verify/request', (route) =>
      route.fulfill({
        status: 429,
        headers: { 'Retry-After': '30' },
        json: { error: { code: 'rate_limited', message: 'Demasiadas solicitudes' } },
      }),
    );
    await showBanner(page);
    await page.getByRole('button', { name: 'Reenviar correo' }).click();
    await expect(page.getByTestId('verify-email-feedback')).toContainText('demasiados correos');

    await page.getByRole('button', { name: 'Cerrar el aviso de verificación' }).click();
    await expect(page.getByTestId(BANNER)).toHaveCount(0);
  });
});
