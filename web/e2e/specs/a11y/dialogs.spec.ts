import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { axeSummary } from '../../lib/axe.ts';
import { appPath } from '../../lib/paths.ts';
import { fillRegisterSteps } from '../../lib/register.ts';
import { waitForReady } from '../../lib/wait.ts';
import { projectWithIssue } from '../issues/issue-fixtures.ts';

// Axe on every kind of open overlay: dialogs, confirmation, legal documents, shortcuts, command palette,
// notes drawer and menus. Findings of all overlays of a test are reported together.

/** Presses a shortcut until the shell (which registers it after the first render) answers. */
async function pressUntilVisible(
  page: Page,
  press: () => Promise<void>,
  target: ReturnType<Page['getByRole']>,
): Promise<void> {
  await expect(async () => {
    await page.locator('body').click({ position: { x: 5, y: 5 } });
    await press();
    await expect(target).toBeVisible({ timeout: 1000 });
  }).toPass();
}

async function settle(page: Page): Promise<void> {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.waitForTimeout(300);
}

test.describe('accesibilidad: dialogos y superposiciones', () => {
  test('crear incidencia, paleta, atajos, apuntes y menu de cuenta', async ({ as, api, data }) => {
    test.setTimeout(180_000);
    const { project } = await projectWithIssue(api, data, 'axd');
    const page = await as('member');
    await page.goto(appPath(`/projects/${project.key}/board`));
    await waitForReady(page);
    const failures: string[] = [];
    const scan = async (name: string): Promise<void> => {
      await settle(page);
      const summary = await axeSummary(page, { name });
      if (summary) failures.push(summary);
    };

    // Create issue dialog.
    await page.getByRole('button', { name: 'Crear', exact: true }).click();
    const create = page.getByRole('dialog', { name: 'Crear incidencia' });
    await expect(create).toBeVisible();
    await scan('dialogo crear incidencia');
    await page.keyboard.press('Escape');
    await expect(create).toBeHidden();

    // Command palette.
    const palette = page.getByRole('dialog', { name: 'Paleta de comandos' });
    await pressUntilVisible(page, () => page.keyboard.press('Control+K'), palette);
    await palette.getByLabel('Buscar o escribir un comando').fill('Incidencias');
    await scan('paleta de comandos');
    await page.keyboard.press('Escape');
    await expect(palette).toBeHidden();

    // Keyboard shortcuts dialog.
    const shortcuts = page.getByRole('dialog', { name: 'Atajos de teclado' });
    await pressUntilVisible(page, () => page.keyboard.type('?'), shortcuts);
    await scan('dialogo de atajos');
    await page.keyboard.press('Escape');
    await expect(shortcuts).toBeHidden();

    // Notes drawer (the button is in the top bar of every work page).
    const notesButton = page.getByRole('button', { name: 'Mis apuntes' });
    if (await notesButton.isVisible()) {
      await notesButton.click();
      await expect(page.getByRole('dialog', { name: /apuntes/i })).toBeVisible();
      await scan('cajon de apuntes');
      await page.keyboard.press('Escape');
    }

    // Account menu.
    await page.getByRole('button', { name: /^Menú de / }).click();
    await expect(page.getByRole('menu').first()).toBeVisible();
    await scan('menu de cuenta');
    await page.keyboard.press('Escape');

    expect(failures.join('\n'), failures.join('\n')).toBe('');
  });

  test('confirmacion de eliminar proyecto', async ({ as, api, data }) => {
    const { project } = await projectWithIssue(api, data, 'axc');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/danger`));
    await waitForReady(page);
    const dialog = page.getByRole('dialog');
    await expect(async () => {
      await page.getByTestId('danger-delete').getByRole('button').click();
      await expect(dialog).toBeVisible({ timeout: 2000 });
    }).toPass();
    const summary = await axeSummary(page, { name: 'confirmacion de eliminar proyecto' });
    expect(summary, summary).toBe('');
  });

  test('documentos legales del registro', async ({ page }) => {
    await page.goto(appPath('/register'));
    await fillRegisterSteps(page, {
      firstName: 'Ana',
      lastName: 'Torres',
      phone: '+51 912 345 678',
      email: 'ana.axe@corelog.test',
      password: 'Secreta-12345',
    });
    await page.waitForTimeout(400);
    await page.getByRole('button', { name: 'Política de privacidad', exact: true }).first().click();
    await expect(page.getByRole('dialog', { name: 'Política de privacidad' })).toBeVisible();
    await settle(page);
    const summary = await axeSummary(page, { name: 'documento legal' });
    expect(summary, summary).toBe('');
  });
});
