import { appPath } from '../../lib/paths.ts';
import type { Page } from '@playwright/test';

import { orgIdBySlug } from '../../lib/api.ts';
import { loginViaUi } from '../../lib/auth.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { skipVisualOutsideLinux } from '../../lib/visual.ts';
import { waitForImages } from '../../lib/wait.ts';
import { expect, test, type OpenAsRole } from '../../fixtures/test.ts';

/** Sidebar of the project context: the only context with an expandable side menu (ContextSidebar). */
const sideMenu = (page: Page) => page.getByRole('navigation', { name: /^Navegación:/ });

/** Opens a project board as the role and returns the page. The project comes from the API. */
async function openProjectBoard(
  as: OpenAsRole,
  role: 'admin' | 'owner',
  api: (r: 'admin' | 'owner') => Promise<import('@playwright/test').APIRequestContext>,
): Promise<Page> {
  const client = await api(role);
  const orgId = await orgIdBySlug(client, role === 'admin' ? 'corelog' : 'acme');
  const listed = (await (
    await client.get('projects', { params: { organization_id: orgId } })
  ).json()) as { data: Array<{ id: string }> };
  const projectId = listed.data[0]?.id;
  expect(projectId, 'the organization must have a project').toBeTruthy();
  const page = await as(role);
  await page.goto(appPath(`/projects/${projectId}/board`));
  await waitForImages(page);
  return page;
}

test.describe('navegacion superior y cuenta', () => {
  test('la navegacion superior muestra las secciones del producto', async ({ as }) => {
    const page = await as('owner');
    await page.goto(appPath('/issues'));
    test.skip(
      await page.getByRole('navigation', { name: 'Principal' }).isHidden(),
      'top menu is in the drawer on narrow viewports',
    );
    const top = page.getByRole('navigation', { name: 'Principal' });
    for (const label of ['Proyectos', 'Filtros', 'Paneles', 'Equipos']) {
      await expect(top.getByRole('button', { name: label })).toBeVisible();
    }
    await expect(page.getByRole('button', { name: 'Crear' })).toBeVisible();
  });

  test('el selector de organizacion muestra la organizacion activa', async ({ as }, testInfo) => {
    test.skip(
      (testInfo.project.use.viewport?.width ?? 0) < 768,
      'the organization switcher is not in the top bar on phones',
    );
    const page = await as('owner');
    await page.goto(appPath('/issues'));
    await expect(page.getByRole('button', { name: /^Organización actual/ })).toHaveAccessibleName(
      /Acme/,
    );
  });

  test('cambiar de organizacion persiste la seleccion tras recargar', async ({ as }, testInfo) => {
    test.skip(
      (testInfo.project.use.viewport?.width ?? 0) < 768,
      'the organization switcher is not in the top bar on phones',
    );
    const page = await as('admin');
    await page.goto(appPath('/issues'));
    await expect(page.getByRole('button', { name: /^Organización actual/ })).toHaveAccessibleName(
      /CoreLog/,
    );
    await page.getByRole('button', { name: /^Organización actual/ }).click();
    await page.getByRole('menuitemradio', { name: /Acme/ }).click();
    await expect(page.getByRole('button', { name: /^Organización actual/ })).toHaveAccessibleName(
      /Acme/,
    );
    await page.reload();
    await expect(page.getByRole('button', { name: /^Organización actual/ })).toHaveAccessibleName(
      /Acme/,
    );
  });

  test('menu de usuario: tema y densidad persisten tras recargar', async ({
    browser,
    data,
  }, testInfo) => {
    // Preferences are per account: a private account keeps parallel viewport runs from toggling the shared ones.
    const user = await data.registerUser('prefs');
    const created = await fetch(`${readE2eEnv().apiURL}/organizations`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${user.token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: data.uniqueName('prefs') }),
    });
    const orgId = ((await created.json()) as { data: { id: string } }).data.id;
    const context = await browser.newContext({ viewport: testInfo.project.use.viewport ?? null });
    const page = await context.newPage();
    await loginViaUi(page, user.email, user.password);
    await page.goto(appPath('/issues'));
    const menu = page.getByRole('button', { name: /^Menú de / });
    const darkSelected = async () => {
      await menu.click();
      const checked = await page
        .getByRole('menuitemradio', { name: 'Oscuro' })
        .getAttribute('aria-checked');
      await page.keyboard.press('Escape');
      return checked;
    };
    const compactSelected = async () => {
      await menu.click();
      const checked = await page
        .getByRole('menuitemcheckbox', { name: /Densidad compacta/ })
        .getAttribute('aria-checked');
      await page.keyboard.press('Escape');
      return checked;
    };

    expect(await darkSelected()).toBe('false');
    expect(await compactSelected()).toBe('false');
    try {
      await menu.click();
      await page.getByRole('menuitemradio', { name: 'Oscuro' }).click();
      await page.getByRole('menuitemcheckbox', { name: /Densidad compacta/ }).click();
      await page.keyboard.press('Escape');

      await page.reload();
      expect(await darkSelected()).toBe('true');
      expect(await compactSelected()).toBe('true');
    } finally {
      await context.close();
      await fetch(`${readE2eEnv().apiURL}/organizations/${orgId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${user.token}` },
      });
    }
  });

  test('el enlace "Saltar al contenido" lleva el foco al contenido principal', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/issues'));
    // The skip link is the first focusable element of the shell; wait until the session check has finished.
    await page.getByText('Saltar al contenido', { exact: true }).waitFor({ state: 'attached' });
    await page.keyboard.press('Tab');
    const focused = await page.evaluate(() => document.activeElement?.textContent?.trim() ?? '');
    expect(focused).toBe('Saltar al contenido');
    await page.keyboard.press('Enter');
    await expect(page.locator('#main-content')).toBeVisible();
    await expect(page.getByRole('main')).toBeVisible();
  });
});

test.describe('barra lateral de contexto', () => {
  test.skip(
    ({ viewport }) => (viewport?.width ?? 0) < 1280,
    'the side menu is expanded only at desktop widths',
  );

  test('el menu lateral se contrae y el estado persiste tras recargar', async ({ as, api }) => {
    const page = await openProjectBoard(as, 'owner', api);
    const menu = sideMenu(page);
    await expect(menu).toBeVisible();
    await page.getByRole('button', { name: 'Contraer menú lateral' }).click();
    await expect(page.getByRole('button', { name: 'Expandir menú lateral' })).toBeVisible();

    await page.reload();
    await expect(page.getByRole('button', { name: 'Expandir menú lateral' })).toBeVisible();
    await page.getByRole('button', { name: 'Expandir menú lateral' }).click();
    await expect(page.getByRole('button', { name: 'Contraer menú lateral' })).toBeVisible();
  });

  test.describe('@visual encabezado de marca', () => {
    skipVisualOutsideLinux();

    test('expandido: banner de marca', async ({ as, api }) => {
      const page = await openProjectBoard(as, 'owner', api);
      await expect(page.getByRole('img', { name: 'CoreLog' }).first()).toHaveScreenshot(
        'sidebar-brand-expanded.png',
        { maxDiffPixels: 100 },
      );
    });
  });
});

test.describe('navegacion movil', () => {
  test.skip(
    ({ viewport }) => (viewport?.width ?? 0) >= 768,
    'drawer navigation is for narrow viewports',
  );

  test('el boton de menu abre el cajon de navegacion y se cierra', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/issues'));
    await page.getByRole('button', { name: 'Abrir menú de navegación' }).click();
    const drawer = page.getByRole('navigation', { name: 'Navegación móvil' });
    await expect(drawer).toBeVisible();
    await expect(
      drawer
        .getByRole('link', { name: 'Proyectos' })
        .or(drawer.getByRole('button', { name: 'Proyectos' })),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(drawer).toBeHidden();
  });
});

test.describe('busqueda y atajos de teclado', () => {
  test('Ctrl+K abre la paleta, busca, navega y Esc la cierra', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/dashboard'));
    await page.locator('body').click({ position: { x: 5, y: 5 } });
    await page.keyboard.press('Control+K');
    const palette = page.getByRole('dialog', { name: 'Paleta de comandos' });
    await expect(palette).toBeVisible();

    await palette.getByLabel('Buscar o escribir un comando').fill('Ir a Incidencias');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/issues$/);

    await page.keyboard.press('Control+K');
    await expect(palette).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(palette).toBeHidden();
  });

  test('el atajo ? muestra los atajos de teclado y Esc los cierra', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/issues'));
    await page.locator('body').click({ position: { x: 5, y: 5 } });
    await page.keyboard.type('?');
    const dialog = page.getByRole('dialog', { name: 'Atajos de teclado' });
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  });

  test('las secuencias g i y g p navegan a incidencias y proyectos', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/dashboard'));
    await page.locator('body').click({ position: { x: 5, y: 5 } });
    await page.keyboard.press('g');
    await page.keyboard.press('p');
    await expect(page).toHaveURL(/\/projects$/);
    await page.keyboard.press('g');
    await page.keyboard.press('i');
    await expect(page).toHaveURL(/\/issues$/);
  });
});
