import { appPath } from '../../lib/paths.ts';
import { readE2eEnv } from '../../lib/env.ts';
import {
  classifyViolations,
  loadKnownViolations,
  writeObserved,
  ruleIdOf,
} from '../../lib/known-violations.ts';
import { collectLayoutAudit } from '../../lib/layout-audit.ts';
import { skipVisualOutsideLinux } from '../../lib/visual.ts';
import { waitForImages } from '../../lib/wait.ts';
import { expect, test } from '../../fixtures/test.ts';

test.describe('pantalla de login', () => {
  test('muestra el formulario de acceso y el panel de marca', async ({ page }) => {
    await page.goto(appPath('/login'));
    await waitForImages(page);

    await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
    await expect(page.getByLabel('Correo electrónico')).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Contraseña' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Crear una' })).toBeVisible();
  });

  test('inicia sesion con credenciales validas y llega a incidencias', async ({ page }) => {
    const { password } = readE2eEnv();
    test.skip(!password, 'E2E_USER_PASSWORD is not set');

    await page.goto(appPath('/login'));
    await page.getByLabel('Correo electrónico').fill('admin@corelog.local');
    await page.getByRole('textbox', { name: 'Contraseña' }).fill(password ?? '');
    await page.getByRole('button', { name: 'Iniciar sesión' }).click();

    await expect(page).toHaveURL(/\/(issues|dashboard)/);
  });

  test('la pantalla de login cumple la auditoria de layout', async ({ page }, testInfo) => {
    await page.goto(appPath('/login'));
    await waitForImages(page);
    // The audit measures the settled form: wait until the sign-in heading has rendered.
    await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();

    // Same known-violations baseline as the smoke journey, keyed "login|<viewport>".
    const result = await collectLayoutAudit(page, { template: 'split', name: 'login' });
    const key = `login|${testInfo.project.name}`;
    const split = classifyViolations(key, result.violations, loadKnownViolations());
    const summary = [
      ...split.known.map((item) => `known  ${key} ${item}`),
      ...split.fixed.map((rule) => `fixed  ${key} ${rule} (remove from baseline)`),
    ];
    await testInfo.attach('known-violations-summary.txt', {
      body: summary.join('\n') || 'none',
      contentType: 'text/plain',
    });

    if (process.env.E2E_UPDATE_KNOWN === '1') {
      writeObserved(`login-${testInfo.project.name}`, {
        [key]: [...new Set(result.violations.map(ruleIdOf))].sort(),
      });
      return;
    }
    expect(split.fresh, `new layout violations on login (${testInfo.project.name})`).toEqual([]);
  });

  test.describe('@visual captura de login', () => {
    skipVisualOutsideLinux();

    test('banner y formulario coinciden con la linea base', async ({ page }) => {
      await page.goto(appPath('/login'));
      await waitForImages(page);

      await expect(page).toHaveScreenshot('login.png', { fullPage: true });
    });
  });
});
