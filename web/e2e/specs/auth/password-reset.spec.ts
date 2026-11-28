import { appPath } from '../../lib/paths.ts';
import { THEME_STORAGE_KEY } from '../../lib/storage-state.ts';
import { waitForImages } from '../../lib/wait.ts';
import { expect, test } from '../../fixtures/test.ts';

// The reset and verification tokens travel by email, so the specs intercept the requests that carry them.
// The seeded accounts are never touched.

test.describe('recuperación de contraseña y verificación de correo', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    const theme = testInfo.project.name.endsWith('-dark') ? 'dark' : 'light';
    await page.addInitScript(
      ([key, value]: string[]) => {
        if (window.localStorage.getItem(key) === null) window.localStorage.setItem(key, value);
      },
      [THEME_STORAGE_KEY, theme],
    );
  });

  test('el acceso ofrece el enlace para recuperar la contraseña', async ({ page }) => {
    await page.goto(appPath('/login'));
    await page.getByRole('link', { name: '¿Olvidaste tu contraseña?' }).click();
    await expect(page).toHaveURL(/\/forgot-password$/);
    await expect(page.getByRole('heading', { name: 'Recupera tu contraseña' })).toBeVisible();
  });

  test('pedir el enlace muestra siempre el mismo mensaje y una cuenta atrás', async ({ page }) => {
    await page.goto(appPath('/forgot-password'));
    await waitForImages(page);
    await page.getByLabel('Correo electrónico').fill('nadie-existe@example.com');
    await page.getByTestId('forgot-submit').getByRole('button').click();
    await expect(page.getByTestId('forgot-sent')).toContainText('Si el correo está registrado');
    await expect(page.getByTestId('forgot-resend')).toContainText('Reenviar en');
    await expect(page.getByTestId('forgot-resend').getByRole('button')).toBeDisabled();
  });

  test('un correo inválido no se envía', async ({ page }) => {
    let calls = 0;
    await page.route('**/auth/password/forgot', async (route) => {
      calls += 1;
      await route.fulfill({ status: 202, json: { message: 'ok' } });
    });
    await page.goto(appPath('/forgot-password'));
    await page.getByLabel('Correo electrónico').fill('no-es-un-correo');
    await page.getByTestId('forgot-submit').getByRole('button').click();
    await expect(page.getByText('Introduce un correo electrónico válido')).toBeVisible();
    expect(calls).toBe(0);
  });

  test('restablecer: contraseña distinta, débil y correcta', async ({ page }) => {
    let attempt = 0;
    await page.route('**/auth/password/reset', async (route) => {
      attempt += 1;
      if (attempt === 1) {
        await route.fulfill({
          status: 422,
          json: { error: { code: 'user.password_weak', message: 'weak' } },
        });
      } else {
        await route.fulfill({ status: 204 });
      }
    });
    await page.goto(appPath('/reset-password?token=token-de-prueba'));
    await waitForImages(page);
    const fields = page.locator('input[type="password"]');
    await fields.nth(0).fill('Clave-Segura-2026!');
    await fields.nth(1).fill('otra-clave');
    await page.getByTestId('reset-submit').getByRole('button').click();
    await expect(page.getByText('Las contraseñas no coinciden').first()).toBeVisible();

    await fields.nth(1).fill('Clave-Segura-2026!');
    await page.getByTestId('reset-submit').getByRole('button').click();
    await expect(page.getByText('demasiado débil')).toBeVisible();

    await page.getByTestId('reset-submit').getByRole('button').click();
    await expect(page.getByTestId('reset-done')).toBeVisible();
    await expect(page.getByTestId('reset-login')).toHaveAttribute('href', '/login');
  });

  test('un enlace vencido ofrece pedir otro', async ({ page }) => {
    await page.route('**/auth/password/reset', (route) =>
      route.fulfill({
        status: 400,
        json: { error: { code: 'auth.reset_token_invalid', message: 'invalid' } },
      }),
    );
    await page.goto(appPath('/reset-password?token=viejo'));
    const fields = page.locator('input[type="password"]');
    await fields.nth(0).fill('Clave-Segura-2026!');
    await fields.nth(1).fill('Clave-Segura-2026!');
    await page.getByTestId('reset-submit').getByRole('button').click();
    await expect(page.getByTestId('reset-invalid')).toBeVisible();
    await page.getByTestId('reset-again').click();
    await expect(page).toHaveURL(/\/forgot-password$/);
  });

  test('sin token el enlace de restablecer es inválido', async ({ page }) => {
    await page.goto(appPath('/reset-password'));
    await expect(page.getByTestId('reset-invalid')).toBeVisible();
  });

  test('verificar el correo: éxito y error', async ({ page }) => {
    await page.route('**/auth/email/verify', async (route) => {
      const body = route.request().postDataJSON() as { token: string };
      if (body.token === 'bueno') await route.fulfill({ status: 204 });
      else
        await route.fulfill({
          status: 400,
          json: { error: { code: 'auth.verification_token_invalid', message: 'invalid' } },
        });
    });
    await page.goto(appPath('/verify-email?token=bueno'));
    await expect(page.getByTestId('verify-success')).toBeVisible();
    await page.goto(appPath('/verify-email?token=malo'));
    await expect(page.getByTestId('verify-error')).toBeVisible();
  });

  test('el inicio de sesión explica el límite de intentos con la espera', async ({ page }) => {
    await page.route('**/auth/login', (route) =>
      route.fulfill({
        status: 429,
        headers: { 'Retry-After': '120' },
        json: { error: { code: 'auth.rate_limited', message: 'too many' } },
      }),
    );
    await page.goto(appPath('/login'));
    await page.getByLabel('Correo electrónico').fill('alguien@example.com');
    await page.getByRole('textbox', { name: 'Contraseña' }).fill('cualquier-cosa');
    await page.getByRole('button', { name: 'Iniciar sesión' }).click();
    await expect(page.getByRole('alert').first()).toContainText('2 minutos');
  });

  test('la cuenta bloqueada y el correo sin verificar tienen su mensaje', async ({ page }) => {
    let calls = 0;
    await page.route('**/auth/login', (route) => {
      calls += 1;
      return route.fulfill(
        calls === 1
          ? {
              status: 423,
              headers: { 'Retry-After': '900' },
              json: { error: { code: 'auth.account_locked', message: 'locked' } },
            }
          : {
              status: 403,
              json: { error: { code: 'auth.email_not_verified', message: 'verify' } },
            },
      );
    });
    await page.goto(appPath('/login'));
    await page.getByLabel('Correo electrónico').fill('alguien@example.com');
    await page.getByRole('textbox', { name: 'Contraseña' }).fill('cualquier-cosa');
    await page.getByRole('button', { name: 'Iniciar sesión' }).click();
    await expect(page.getByRole('alert').first()).toContainText('bloqueada');
    await page.getByRole('button', { name: 'Iniciar sesión' }).click();
    await expect(page.getByRole('alert').first()).toContainText('no está verificado');
  });

  test('las tres pantallas cumplen la auditoría de layout', async ({ page, audit }) => {
    for (const [path, name] of [
      ['/forgot-password', 'forgot-password'],
      ['/reset-password?token=x', 'reset-password'],
      ['/verify-email', 'verify-email'],
    ] as const) {
      await page.goto(appPath(path));
      await waitForImages(page);
      await audit(page, { template: 'split', name });
    }
  });
});
