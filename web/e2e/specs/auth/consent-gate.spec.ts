import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { loginViaUi } from '../../lib/auth.ts';
import { readE2eEnv } from '../../lib/env.ts';

/**
 * Publishing a new version would put every account (including the seeded ones other specs use at the same
 * time) in front of the gate. Instead the answers of the throwaway account carry a pending document until
 * the real acceptance is posted, which exercises the same screen and the real POST /me/consents.
 */
async function pendOnePolicy(page: Page): Promise<{ accepted: () => boolean }> {
  let accepted = false;
  await page.route('**/users/me', async (route) => {
    const response = await route.fetch();
    const body = (await response.json()) as { data: Record<string, unknown> };
    body.data['pending_consents'] = accepted
      ? []
      : [{ slug: 'privacy_policy', title: 'Política de privacidad', version: 1 }];
    await route.fulfill({ response, json: body });
  });
  await page.route('**/me/consents', async (route) => {
    if (route.request().method() === 'POST') accepted = true;
    await route.continue();
  });
  return { accepted: () => accepted };
}

test.describe('nueva versión de un documento legal', () => {
  test('bloquea la sesión hasta aceptar, no se cierra con Escape y se puede salir', async ({
    browser,
    data,
  }) => {
    const user = await data.registerUser('gate');
    const context = await browser.newContext();
    const page = await context.newPage();
    const state = await pendOnePolicy(page);
    try {
      await loginViaUi(page, user.email, user.password);
      const gate = page.getByRole('dialog', { name: 'Actualizamos nuestros documentos legales' });
      await expect(gate).toBeVisible();
      await expect(gate.getByText(/versión 1/)).toBeVisible();

      // Not dismissable: no close button, Escape and a click outside do nothing.
      await expect(gate.getByRole('button', { name: 'Cerrar', exact: true })).toHaveCount(0);
      await page.keyboard.press('Escape');
      await page.mouse.click(5, 5);
      await expect(gate).toBeVisible();
      const accept = gate.getByRole('button', { name: 'Aceptar y continuar' });
      await expect(accept).toBeDisabled();

      // The document opens in its own modal and accepting there ticks the box.
      await gate.getByRole('button', { name: 'Política de privacidad' }).click();
      const reader = page.getByRole('dialog', { name: 'Política de privacidad' });
      await expect(reader).toBeVisible();
      await reader.getByRole('button', { name: 'Aceptar y cerrar' }).click();
      await expect(gate.getByRole('checkbox', { name: /Política de privacidad/ })).toBeChecked();

      await accept.click();
      await expect(gate).toBeHidden();
      expect(state.accepted()).toBe(true);

      const consents = await fetch(`${readE2eEnv().apiURL}/me/consents`, {
        headers: { Authorization: `Bearer ${user.token}` },
      });
      const { data: status } = (await consents.json()) as {
        data: { accepted: Array<{ slug: string; version: number }> };
      };
      expect(status.accepted).toContainEqual(expect.objectContaining({ slug: 'privacy_policy' }));
    } finally {
      await context.close();
    }
  });

  test('"Cerrar sesión" desde el aviso termina la sesión', async ({ browser, data }) => {
    const user = await data.registerUser('gate-out');
    const context = await browser.newContext();
    const page = await context.newPage();
    await pendOnePolicy(page);
    try {
      await loginViaUi(page, user.email, user.password);
      const gate = page.getByRole('dialog', { name: 'Actualizamos nuestros documentos legales' });
      await expect(gate).toBeVisible();
      await gate.getByRole('button', { name: 'Cerrar sesión' }).click();
      await expect(gate).toBeHidden();
      expect(
        await page.evaluate(() => window.localStorage.getItem('corelog.auth.token')),
      ).toBeNull();
    } finally {
      await context.close();
    }
  });
});
