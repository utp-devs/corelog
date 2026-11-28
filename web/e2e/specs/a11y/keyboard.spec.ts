import type { Locator, Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { cardKeysByColumn, mainBoardId } from '../../lib/agile-api.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { readE2eEnv } from '../../lib/env.ts';
import { appPath } from '../../lib/paths.ts';
import { createProjectViaApi, randomProjectKey } from '../../lib/projects-api.ts';
import { createIssueViaApi } from '../../lib/workflow-api.ts';
import { roles } from '../../fixtures/roles.ts';
import { projectWithIssue } from '../issues/issue-fixtures.ts';

// Keyboard-only walkthroughs: Tab, Shift+Tab, Enter, Space, arrows and Escape. Nothing here clicks or hovers:
// the only pointer-free way in is the keyboard, so a control that cannot be reached or operated fails the test.

const isFocused = (target: Locator): Promise<boolean> =>
  target.evaluate((el) => el === document.activeElement).catch(() => false);

/** Presses Tab (or Shift+Tab) until `target` has focus; fails when it is not reachable in `max` presses. */
async function tabTo(page: Page, target: Locator, max = 120, backwards = false): Promise<void> {
  const key = backwards ? 'Shift+Tab' : 'Tab';
  for (let step = 0; step < max; step += 1) {
    if (await isFocused(target)) return;
    await page.keyboard.press(key);
  }
  expect(await isFocused(target), `${String(target)} is reachable with ${key}`).toBe(true);
}

/** The element with focus is inside `scope`. */
async function expectFocusInside(scope: Locator, message: string): Promise<void> {
  await expect(
    scope.evaluate((el) => el.contains(document.activeElement)),
    message,
  ).resolves.toBe(true);
}

/** Presses a shortcut until the shell (which registers it after the first render) answers. */
async function pressUntilVisible(
  page: Page,
  press: () => Promise<void>,
  target: Locator,
): Promise<void> {
  await expect(async () => {
    await page.locator('body').focus();
    await press();
    await expect(target).toBeVisible({ timeout: 1000 });
  }).toPass();
}

/** Focus stays inside `dialog` for a full Tab cycle and a full Shift+Tab cycle. */
async function expectFocusTrapped(page: Page, dialog: Locator): Promise<void> {
  for (let step = 0; step < 40; step += 1) {
    await page.keyboard.press('Tab');
    await expectFocusInside(dialog, `Tab ${step + 1} stays inside the dialog`);
  }
  for (let step = 0; step < 40; step += 1) {
    await page.keyboard.press('Shift+Tab');
    await expectFocusInside(dialog, `Shift+Tab ${step + 1} stays inside the dialog`);
  }
}

test.describe('accesibilidad: solo teclado', () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(
      testInfo.project.name !== 'desktop',
      'Keyboard flows run once, on the desktop project',
    );
  });

  test('inicio de sesion', async ({ browser }) => {
    const context = await browser.newContext({ baseURL: readE2eEnv().baseURL });
    const page = await context.newPage();
    await page.goto(appPath('/login'));
    const email = page.locator('input[type="email"]');
    const password = page.locator('input[type="password"]');
    await expect(email).toBeVisible();
    await tabTo(page, email);
    await page.keyboard.type(roles.admin.email);
    await page.keyboard.press('Tab');
    await tabTo(page, password, 5);
    await page.keyboard.type(readE2eEnv().password ?? '');
    await page.keyboard.press('Enter');
    await expect(page).not.toHaveURL(/\/login/);
    await context.close();
  });

  test('asistente de registro: avanzar, retroceder y abrir un documento legal', async ({
    browser,
  }) => {
    const context = await browser.newContext({ baseURL: readE2eEnv().baseURL });
    const page = await context.newPage();
    await page.goto(appPath('/register'));
    const first = page.getByLabel(/^Nombres/);
    await expect(first).toBeVisible();
    await tabTo(page, first);
    await page.keyboard.type('Ana');
    await page.keyboard.press('Tab');
    await page.keyboard.type('Torres');
    await page.keyboard.press('Tab');
    await tabTo(page, page.getByLabel(/^Teléfono/), 6);
    await page.keyboard.type('912345678');
    const next = page.getByTestId('register-next');
    await tabTo(page, next.getByRole('button'), 10);
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('register-step-2')).toBeVisible();

    // Back with the keyboard, then forward again.
    const back = page.getByRole('button', { name: /Atrás|Anterior|Volver/ }).first();
    await tabTo(page, back, 20, true);
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('register-step-1')).toBeVisible();
    await tabTo(page, next.getByRole('button'), 20);
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('register-step-2')).toBeVisible();

    await tabTo(page, page.getByLabel('Correo electrónico'), 10);
    await page.keyboard.type('ana.teclado@corelog.test');
    await page.keyboard.press('Tab');
    await page.keyboard.type('Secreta-12345');
    await tabTo(page, page.getByLabel(/^Confirmar contraseña/), 6);
    await page.keyboard.type('Secreta-12345');
    await tabTo(page, next.getByRole('button'), 10);
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('register-step-3')).toBeVisible();
    await page.waitForTimeout(400);

    // The policy link opens a dialog, Escape closes it and focus comes back to the link.
    const link = page.getByRole('button', { name: 'Política de privacidad', exact: true }).first();
    await tabTo(page, link, 30);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Política de privacidad' });
    await expect(dialog).toBeVisible();
    await expectFocusInside(dialog, 'focus moves into the legal dialog');
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect.poll(() => isFocused(link)).toBe(true);

    // Consent checkboxes are ticked with Space.
    const consent = page.getByRole('checkbox', { name: /Política de privacidad/ });
    await tabTo(page, consent, 10, true);
    await page.keyboard.press('Space');
    await expect(consent).toBeChecked();
    await context.close();
  });

  test('dialogo de crear incidencia: foco atrapado y devuelto al boton', async ({
    as,
    api,
    data,
  }) => {
    const { project } = await projectWithIssue(api, data, 'kbc');
    const page = await as('member');
    await page.goto(appPath(`/projects/${project.key}/board`));
    const opener = page.getByRole('button', { name: 'Crear', exact: true });
    await expect(opener).toBeVisible();
    await tabTo(page, opener);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Crear incidencia' });
    await expect(dialog).toBeVisible();
    await expectFocusInside(dialog, 'focus moves into the dialog on open');
    await expectFocusTrapped(page, dialog);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect.poll(() => isFocused(opener)).toBe(true);
  });

  test('tablero: mover una tarjeta de columna con Alt+Flecha', async ({ as, api, data }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await createProjectViaApi(
      owner,
      acme,
      data.uniqueName('kbb'),
      randomProjectKey(),
    );
    data.trackProject('owner', project);
    const issue = await createIssueViaApi(owner, project.id, 'Tarjeta del teclado');
    const boardId = await mainBoardId(owner, project.id);
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/board`));
    const link = page.getByTestId(`kanban-card-${issue.key}`).getByRole('link');
    await expect(link).toBeVisible();
    await tabTo(page, link);
    await page.keyboard.press('Alt+ArrowRight');
    await expect(
      page.getByRole('status').filter({ hasText: `${issue.key} movida a` }),
    ).toBeVisible();
    await expect.poll(async () => (await cardKeysByColumn(owner, boardId))[1]).toContain(issue.key);
    // The card keeps keyboard focus after the move so the person can keep moving it.
    await expect
      .poll(() =>
        page.evaluate(
          () => document.activeElement?.closest('[data-testid^="kanban-card-"]') !== null,
        ),
      )
      .toBe(true);
  });

  test('backlog: seleccionar varias incidencias con el teclado', async ({ as, api, data }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'kbl');
    const one = await createIssueViaApi(owner, project.id, 'Primera del backlog');
    const two = await createIssueViaApi(owner, project.id, 'Segunda del backlog');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/backlog`));
    const rowOne = page.getByTestId(`backlog-row-${one.key}`);
    const rowTwo = page.getByTestId(`backlog-row-${two.key}`);
    await expect(rowOne).toBeVisible();
    await tabTo(page, rowOne);
    await page.keyboard.press('Space');
    await expect(rowOne).toHaveClass(/backlog-row--selected/);
    await tabTo(page, rowTwo);
    await page.keyboard.press('Control+Space');
    await expect(rowTwo).toHaveClass(/backlog-row--selected/);
    await expect(rowOne).toHaveClass(/backlog-row--selected/);
    // The checkbox of a row is a real checkbox: Space toggles it.
    const check = rowTwo.getByRole('checkbox');
    await tabTo(page, check, 20);
    await page.keyboard.press('Space');
    await expect(rowTwo).not.toHaveClass(/backlog-row--selected/);
  });

  test('editor del flujo: abrir una transicion y editarla con el teclado', async ({
    as,
    api,
    data,
  }) => {
    const owner = await api('owner');
    const acme = await orgIdBySlug(owner, 'acme');
    const project = await data.createProject('owner', acme, 'kbw');
    const page = await as('owner');
    await page.goto(appPath(`/projects/${project.key}/settings/workflow`));
    await expect(page.getByTestId('workflow-status').first()).toBeVisible();
    const select = page
      .getByTestId('workflow-transition')
      .first()
      .getByRole('button', { name: /./ })
      .first();
    await tabTo(page, select, 150);
    await page.keyboard.press('Enter');
    const panel = page.getByTestId('workflow-transition-panel');
    await expect(panel).toBeVisible();
    await expect(select).toHaveAttribute('aria-pressed', 'true');
    const name = page.getByTestId('workflow-transition-name');
    await tabTo(page, name, 40);
    await page.keyboard.press('End');
    await page.keyboard.type(' revisada');
    await expect(page.getByTestId('workflow-dirty')).toBeVisible();
    // Space activates the table buttons too.
    await tabTo(page, select, 60, true);
    await page.keyboard.press('Space');
    await expect(select).toHaveAttribute('aria-pressed', 'true');
  });

  test('paleta de comandos: abrir, elegir con flechas y cerrar con Escape', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/dashboard'));
    await expect(page.getByRole('main')).toBeVisible();
    const palette = page.getByRole('dialog', { name: 'Paleta de comandos' });
    await pressUntilVisible(page, () => page.keyboard.press('Control+K'), palette);
    const input = palette.getByLabel('Buscar o escribir un comando');
    await expect.poll(() => isFocused(input)).toBe(true);
    await page.keyboard.type('Ir a');
    await expect(palette.getByRole('option').first()).toBeVisible();
    await page.keyboard.press('ArrowDown');
    await expect(input).toHaveAttribute('aria-activedescendant', /.+/);
    await expectFocusTrapped(page, palette);
    await page.keyboard.press('Escape');
    await expect(palette).toBeHidden();
    await page.keyboard.press('Control+K');
    await expect(palette).toBeVisible();
    await page.keyboard.type('Ir a Incidencias');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/issues$/);
  });

  test('atajos y apuntes: foco atrapado y devuelto', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/issues'));
    await expect(page.getByRole('main')).toBeVisible();
    const shortcuts = page.getByRole('dialog', { name: 'Atajos de teclado' });
    await pressUntilVisible(page, () => page.keyboard.type('?'), shortcuts);
    await expectFocusInside(shortcuts, 'focus moves into the shortcuts dialog');
    await expectFocusTrapped(page, shortcuts);
    await page.keyboard.press('Escape');
    await expect(shortcuts).toBeHidden();

    // Notes drawer from its button, closed with Escape, focus back on the button.
    const notes = page.getByRole('button', { name: 'Mis apuntes' });
    await tabTo(page, notes, 60);
    await page.keyboard.press('Enter');
    const drawer = page.getByRole('dialog', { name: /apuntes/i });
    await expect(drawer).toBeVisible();
    await expectFocusInside(drawer, 'focus moves into the notes drawer');
    await expectFocusTrapped(page, drawer);
    await page.keyboard.press('Escape');
    await expect(drawer).toBeHidden();
    await expect.poll(() => isFocused(notes)).toBe(true);
  });

  test('enlace de salto, foco al cambiar de ruta y avisos', async ({ as }) => {
    const page = await as('member');
    await page.goto(appPath('/issues'));
    const main = page.locator('main#main-content');
    await expect(main).toBeVisible();
    await page.locator('body').focus();
    const skip = page.getByRole('link', { name: 'Saltar al contenido' });
    // The skip link is the first stop of the page and is visible while it has focus.
    await page.keyboard.press('Tab');
    await expect(skip).toBeFocused();
    await expect(skip).toBeInViewport();
    await page.keyboard.press('Enter');
    await expect.poll(() => isFocused(main)).toBe(true);

    // A route change moves focus to the main landmark and announces the page in the live region.
    await pressUntilVisible(
      page,
      async () => {
        await page.keyboard.press('g');
        await page.keyboard.press('p');
      },
      page.getByRole('heading', { name: 'Proyectos', level: 1 }),
    );
    await expect(page).toHaveURL(/\/projects$/);
    await expect.poll(() => isFocused(main)).toBe(true);
    await expect(page.locator('[role="status"][aria-live="polite"]').first()).toBeAttached();
    await expect(page.getByText(/^Página: /)).toBeAttached();
  });
});

test.describe('accesibilidad: solo teclado en telefono', () => {
  test('el menu de navegacion es un cajon con foco atrapado', async ({ as }, testInfo) => {
    test.skip(testInfo.project.name !== 'mobile', 'Only the phone layout has the drawer');
    const page = await as('member');
    await page.goto(appPath('/issues'));
    await expect(page.getByRole('main')).toBeVisible();
    const opener = page.getByRole('button', { name: /menú|navegación/i }).first();
    await tabTo(page, opener, 30);
    await page.keyboard.press('Enter');
    const drawer = page.getByRole('dialog').first();
    await expect(drawer).toBeVisible();
    await expectFocusInside(drawer, 'focus moves into the navigation drawer');
    await expectFocusTrapped(page, drawer);
    await page.keyboard.press('Escape');
    await expect(drawer).toBeHidden();
    await expect.poll(() => isFocused(opener)).toBe(true);
  });
});
