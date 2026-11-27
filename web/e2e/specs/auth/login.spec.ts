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
import { THEME_STORAGE_KEY } from '../../lib/storage-state.ts';
import { waitForImages } from '../../lib/wait.ts';
import { expect, test } from '../../fixtures/test.ts';

test.describe('pantalla de login', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    // Without a stored choice the access screens are dark, so each project stores its theme explicitly.
    const theme = testInfo.project.name.endsWith('-dark') ? 'dark' : 'light';
    await page.addInitScript(
      ([key, value]: string[]) => {
        // Only the first load: a reload keeps whatever the page stored since.
        if (window.localStorage.getItem(key) === null) window.localStorage.setItem(key, value);
      },
      [THEME_STORAGE_KEY, theme],
    );
  });

  test('muestra el formulario de acceso y el panel de marca', async ({ page }) => {
    await page.goto(appPath('/login'));
    await waitForImages(page);

    await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
    await expect(page.getByLabel('Correo electrónico')).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Contraseña' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Crear una' })).toBeVisible();
  });

  test('muestra el carrusel de imagenes en pantallas amplias y el fondo en moviles', async ({
    page,
  }) => {
    await page.goto(appPath('/login'));
    await waitForImages(page);

    const carousel = page.getByRole('region', { name: 'Equipos trabajando con CoreLog' });
    const viewport = page.viewportSize();
    if (viewport && viewport.width <= 720) {
      await expect(carousel).toHaveCount(0);
      await expect(page.getByRole('button', { name: 'Iniciar sesión' })).toBeVisible();
      return;
    }
    await expect(carousel).toBeVisible();
    await expect(carousel.getByRole('button', { name: 'Imagen siguiente' })).toBeVisible();
    await expect(carousel.getByRole('button', { name: /^Ir a la imagen/ })).toHaveCount(4);
  });

  test('el boton flotante cambia el tema, que persiste al recargar y tras iniciar sesion', async ({
    page,
  }, testInfo) => {
    const { password } = readE2eEnv();
    const startsDark = testInfo.project.name.endsWith('-dark');
    const html = page.locator('html');
    await page.goto(appPath('/login'));
    await expect(html).toHaveAttribute('data-theme', startsDark ? 'dark' : 'light');

    const toggle = page.getByTestId('theme-toggle');
    await expect(toggle).toHaveAccessibleName(
      startsDark ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro',
    );
    await toggle.click();
    await expect(html).toHaveAttribute('data-theme', startsDark ? 'light' : 'dark');

    await page.reload();
    await expect(html).toHaveAttribute('data-theme', startsDark ? 'light' : 'dark');

    test.skip(!password, 'E2E_USER_PASSWORD is not set');
    await page.getByLabel('Correo electrónico').fill('admin@corelog.local');
    await page.getByRole('textbox', { name: 'Contraseña' }).fill(password ?? '');
    await page.getByRole('button', { name: 'Iniciar sesión' }).click();
    await expect(page).toHaveURL(/\/(issues|dashboard)/);
    await expect(html).toHaveAttribute('data-theme', startsDark ? 'light' : 'dark');
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

  // The three public access screens share one layout: each is audited with the same split template.
  const AUDITED: ReadonlyArray<{ name: string; path: string; heading: string }> = [
    { name: 'login', path: '/login', heading: 'Iniciar sesión' },
    { name: 'register', path: '/register', heading: 'Crear cuenta' },
    {
      name: 'invite',
      path: '/invite/token-que-no-existe-e2e',
      heading: 'Invitación no disponible',
    },
  ];

  for (const screen of AUDITED) {
    test(`la pantalla ${screen.name} cumple la auditoria de layout`, async ({ page }, testInfo) => {
      await page.goto(appPath(screen.path));
      await waitForImages(page);
      // The audit measures the settled form: wait until the heading has rendered.
      await expect(page.getByRole('heading', { name: screen.heading })).toBeVisible();

      // Same known-violations baseline as the smoke journey, keyed "<screen>|<viewport>".
      const result = await collectLayoutAudit(page, { template: 'split', name: screen.name });
      const key = `${screen.name}|${testInfo.project.name}`;
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
        writeObserved(`${screen.name}-${testInfo.project.name}`, {
          [key]: [...new Set(result.violations.map(ruleIdOf))].sort(),
        });
        return;
      }
      expect(
        split.fresh,
        `new layout violations on ${screen.name} (${testInfo.project.name})`,
      ).toEqual([]);
    });
  }

  test.describe('@visual captura de login', () => {
    skipVisualOutsideLinux();

    test('banner y formulario coinciden con la linea base', async ({ page }) => {
      // Reduced motion keeps the carousel on its first photograph, so the capture is deterministic.
      await page.emulateMedia({ reducedMotion: 'reduce' });
      await page.goto(appPath('/login'));
      await waitForImages(page);

      await expect(page).toHaveScreenshot('login.png', { fullPage: true });
    });
  });
});
