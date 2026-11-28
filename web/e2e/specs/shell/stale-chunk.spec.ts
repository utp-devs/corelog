import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';

const RELOAD_KEY = 'corelog.chunk-reload-at';

/** Makes the next lazy chunk request fail, like a chunk that a new build removed from the server. */
async function failNextChunk(page: Page): Promise<void> {
  let armed = true;
  await page.route(/\/chunk-[A-Za-z0-9]+\.js/, async (route) => {
    if (armed) {
      armed = false;
      await route.abort('failed');
      return;
    }
    await route.continue();
  });
}

test.describe('version nueva de la aplicacion', () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(
      (testInfo.project.use.viewport?.width ?? 0) < 1024,
      'the project sidebar lives in the drawer on narrow viewports',
    );
  });

  test('un chunk perdido recarga la pagina una vez y abre Reportes', async ({ as, api, data }) => {
    const owner = await api('owner');
    const project = await data.createProject('owner', await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/board`));
    const sidebar = page.getByRole('complementary', { name: project.name });
    await expect(sidebar).toBeVisible();
    await page.evaluate((key) => sessionStorage.removeItem(key), RELOAD_KEY);

    await failNextChunk(page);
    const reloaded = page.waitForEvent('load');
    await sidebar.getByRole('link', { name: 'Reportes' }).click();
    await reloaded;

    await expect(page).toHaveURL(new RegExp(`/projects/${project.key}/reports`));
    await expect(page.getByTestId('stale-version-notice')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Reportes' }).first()).toBeVisible();
  });

  test('si falla otra vez muestra el aviso con el boton Recargar', async ({ as, api, data }) => {
    const owner = await api('owner');
    const project = await data.createProject('owner', await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/board`));
    const sidebar = page.getByRole('complementary', { name: project.name });
    await expect(sidebar).toBeVisible();
    await page.evaluate((key) => sessionStorage.setItem(key, String(Date.now())), RELOAD_KEY);

    await failNextChunk(page);
    await sidebar.getByRole('link', { name: 'Reportes' }).click();

    const notice = page.getByTestId('stale-version-notice');
    await expect(notice).toBeVisible();
    await expect(notice).toContainText(
      'Hay una versión nueva de la aplicación. Recarga la página.',
    );
    await expect(page.getByTestId('stale-version-reload')).toHaveText('Recargar');
  });
});
