import { appPath } from '../../lib/paths.ts';
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';

import { loginViaUi } from '../../lib/auth.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { expect, test } from '../../fixtures/test.ts';

/** 1x1 PNG, small enough to keep the fixture on disk tiny. The API converts it to 256x256. */
const PNG_BASE64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';

/**
 * The app shell (and therefore /profile) needs an organization: the registered account gets one of its
 * own through the API. It is deleted when the test ends.
 */
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

test.describe('perfil personal (general)', () => {
  test('guarda nombre para mostrar, cargo y los mantiene tras recargar', async ({
    browser,
    data,
  }) => {
    const user = await data.registerUser('profile');
    const orgId = await giveOrganization(user.token, data.uniqueName('perfil'));
    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await loginViaUi(page, user.email, user.password);
      await page.goto(appPath('/profile'));
      await expect(page.getByRole('heading', { name: 'General' }).first()).toBeVisible();

      const displayName = `Perfil ${Math.random().toString(36).slice(2, 6)}`;
      await page.getByLabel('Nombre para mostrar').fill(displayName);
      await page.getByLabel('Cargo').fill('Ingeniera de QA');
      await page.getByRole('button', { name: 'Guardar cambios' }).click();
      await expect(page.getByRole('button', { name: 'Guardar cambios' })).toBeDisabled();

      await page.reload();
      await expect(page.getByLabel('Nombre para mostrar')).toHaveValue(displayName);
      await expect(page.getByLabel('Cargo')).toHaveValue('Ingeniera de QA');
    } finally {
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });

  test('sube una foto de perfil y la quita', async ({ browser, data }, testInfo) => {
    const user = await data.registerUser('avatar');
    const orgId = await giveOrganization(user.token, data.uniqueName('avatar'));
    const context = await browser.newContext();
    const page = await context.newPage();
    const dir = testInfo.outputPath('fixtures');
    mkdirSync(dir, { recursive: true });
    const file = path.join(dir, 'avatar.png');
    writeFileSync(file, Buffer.from(PNG_BASE64, 'base64'));
    try {
      await loginViaUi(page, user.email, user.password);
      await page.goto(appPath('/profile'));

      await page.getByLabel('Elegir foto de perfil').setInputFiles(file);
      await page.getByRole('button', { name: 'Guardar foto' }).click();
      await expect(page.getByRole('button', { name: 'Quitar foto' })).toBeVisible();

      await page.reload();
      await expect(page.getByRole('button', { name: 'Quitar foto' })).toBeVisible();
      await page.getByRole('button', { name: 'Quitar foto' }).click();
      await expect(page.getByRole('button', { name: 'Subir foto' })).toBeVisible();
    } finally {
      await context.close();
      await removeOrganization(user.token, orgId);
    }
  });
});
