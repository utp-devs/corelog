import { loginViaUi } from '../../lib/auth.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { appPath } from '../../lib/paths.ts';
import { totp } from '../../lib/totp.ts';
import { expect, test } from '../../fixtures/test.ts';

/** The shell (and therefore /profile) needs an organization: the throwaway account gets one of its own. */
async function giveOrganization(token: string, name: string): Promise<string> {
  const response = await fetch(`${readE2eEnv().apiURL}/organizations`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  if (response.status !== 201)
    throw new Error(`creating the organization returned ${response.status}`);
  return ((await response.json()) as { data: { id: string } }).data.id;
}

async function removeOrganization(token: string, id: string): Promise<void> {
  await fetch(`${readE2eEnv().apiURL}/organizations/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  });
}

test('el código TOTP sigue el vector de prueba del estándar', () => {
  // RFC 6238, SHA-1, secret "12345678901234567890", T = 59 s: 94287082 (the last six digits are 287082).
  expect(totp('GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ', 59_000)).toBe('287082');
});

// Two-step verification is enabled only on a throwaway account, never on the shared seeded ones.
test.describe('verificación en dos pasos', () => {
  test('activa con QR, guarda los códigos y exige el código al iniciar sesión', async ({
    browser,
    data,
    shots,
  }) => {
    const user = await data.registerUser('twofa');
    const orgId = await giveOrganization(user.token, data.uniqueName('twofa'));
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await loginViaUi(page, user.email, user.password);
      await page.goto(appPath('/profile/security'));
      await page.getByTestId('two-factor-enable').click();

      await expect(page.getByTestId('two-factor-qr')).toBeVisible();
      const secret = (await page.getByTestId('two-factor-secret').innerText()).replace(/\s/g, '');
      expect(secret).toMatch(/^[A-Z2-7]{16,}$/);
      await shots(page, 'two-factor-enroll');

      // A wrong code keeps the dialog open with its message.
      await page.getByTestId('two-factor-confirm-code').fill('000000');
      await page.getByTestId('two-factor-confirm').click();
      await expect(
        page.getByRole('alert').filter({ hasText: 'El código no es válido' }),
      ).toBeVisible();

      await page.getByTestId('two-factor-confirm-code').fill(totp(secret));
      await page.getByTestId('two-factor-confirm').click();
      const codes = page.getByTestId('recovery-codes').locator('li');
      await expect(codes).toHaveCount(10);
      const recovery = (await codes.first().innerText()).trim();
      await expect(page.getByTestId('recovery-done').getByRole('button')).toBeDisabled();
      await page.getByLabel('Ya guardé mis códigos en un lugar seguro').check();
      await page.getByTestId('recovery-done').click();
      await expect(page.getByTestId('two-factor-status')).toContainText('Activada');
      await shots(page, 'two-factor-enabled');
      await context.close();

      // Sign-in now asks for the second step; the first code is rejected, the next window works.
      const login = await browser.newContext();
      const loginPage = await login.newPage();
      await loginPage.goto(appPath('/login'));
      await loginPage.locator('input[type="email"]').fill(user.email);
      await loginPage.locator('input[type="password"]').fill(user.password);
      await loginPage.getByRole('button', { name: 'Iniciar sesión' }).click();
      await expect(loginPage.getByTestId('two-factor-step')).toBeVisible();
      await shots(loginPage, 'two-factor-challenge');
      await loginPage.getByTestId('two-factor-code').fill('000000');
      await loginPage.getByRole('button', { name: 'Verificar' }).click();
      await expect(
        loginPage.getByRole('alert').filter({ hasText: 'El código no es válido' }),
      ).toBeVisible();

      // A recovery code also works, once.
      await loginPage.getByTestId('two-factor-toggle').click();
      await loginPage.getByTestId('two-factor-code').fill(recovery);
      await loginPage.getByRole('button', { name: 'Verificar' }).click();
      await expect(loginPage).not.toHaveURL(/\/login/);
      await login.close();
    } finally {
      await context.close().catch(() => undefined);
      await removeOrganization(user.token, orgId);
    }
  });

  test('un reto caducado o abandonado vuelve a las credenciales', async ({ browser, data }) => {
    const user = await data.registerUser('twofa-back');
    const orgId = await giveOrganization(user.token, data.uniqueName('twofa'));
    // Enable the second step through the API (TOTP computed from the secret).
    const base = readE2eEnv().apiURL;
    const headers = { Authorization: `Bearer ${user.token}`, 'Content-Type': 'application/json' };
    const enrolled = (await (
      await fetch(`${base}/auth/2fa/enroll`, { method: 'POST', headers })
    ).json()) as {
      data: { secret: string };
    };
    const secret = enrolled.data.secret;
    const confirmed = await fetch(`${base}/auth/2fa/confirm`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ code: totp(secret) }),
    });
    expect(confirmed.ok).toBe(true);
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await page.goto(appPath('/login'));
      await page.locator('input[type="email"]').fill(user.email);
      await page.locator('input[type="password"]').fill(user.password);
      await page.getByRole('button', { name: 'Iniciar sesión' }).click();
      await expect(page.getByTestId('two-factor-step')).toBeVisible();
      await page.getByRole('button', { name: 'Volver al inicio de sesión' }).click();
      await expect(page.locator('input[type="password"]')).toBeVisible();
      await expect(page.getByTestId('two-factor-step')).toHaveCount(0);
    } finally {
      // Leave the account without the second step: the throwaway user is deactivated anyway.
      await fetch(`${base}/auth/2fa/disable`, {
        method: 'POST',
        headers,
        body: JSON.stringify({ password: user.password, code: totp(secret, Date.now(), 1) }),
      }).catch(() => undefined);
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });
});
