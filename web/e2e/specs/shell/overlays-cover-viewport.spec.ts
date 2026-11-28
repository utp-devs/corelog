import type { Locator, Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { fillRegisterStep1 } from '../../lib/register.ts';
import { addTransitionViaApi, readWorkflow } from '../../lib/workflow-api.ts';
import { openIssueView, projectWithIssue } from '../issues/issue-fixtures.ts';

// Every dialog, drawer and floating panel of the product is opened and measured: the mask of a modal covers the
// whole viewport, the panel is fully inside it, nothing (an ancestor with overflow, a stacking context, a
// transformed parent) clips or covers its corners, and the document itself never scrolls.

const ACCOUNT = {
  firstName: 'Ana',
  lastName: 'Torres',
  phone: '+51 912 345 678',
  email: 'ana.overlays@corelog.test',
  password: 'Secreta-12345',
};

/** Pixels of slack for sub-pixel layout and the rounded corners of the panels. */
const TOLERANCE = 2;
/** Distance from each corner of the panel where the stacking probe is taken (clear of rounded corners). */
const CORNER_INSET = 8;

interface Geometry {
  readonly viewport: { width: number; height: number };
  readonly panel: { x: number; y: number; width: number; height: number };
  readonly mask: { x: number; y: number; width: number; height: number } | null;
  readonly hidden: string[];
  readonly scroll: { x: number; y: number; scrollWidth: number; scrollHeight: number };
}

/** Measures a panel: its box, the mask behind it (fixed ancestor), what covers its corners and the page scroll. */
async function measure(panel: Locator, inset: number): Promise<Geometry> {
  return panel.evaluate((el, probeInset) => {
    const viewport = { width: window.innerWidth, height: window.innerHeight };
    const box = el.getBoundingClientRect();
    let mask: Geometry['mask'] = null;
    for (let node = el.parentElement; node; node = node.parentElement) {
      if (getComputedStyle(node).position === 'fixed') {
        const r = node.getBoundingClientRect();
        mask = { x: r.x, y: r.y, width: r.width, height: r.height };
        // The outermost fixed ancestor that spans the viewport is the mask.
        if (r.width >= viewport.width - 1 && r.height >= viewport.height - 1) break;
      }
    }
    // A point of the panel must hit the panel (or a child of it): anything else clips or covers it.
    const hidden: string[] = [];
    const left = Math.max(box.x, 0);
    const top = Math.max(box.y, 0);
    const right = Math.min(box.x + box.width, viewport.width);
    const bottom = Math.min(box.y + box.height, viewport.height);
    const probes: Array<[string, number, number]> = [
      ['top-left', left + probeInset, top + probeInset],
      ['top-right', right - probeInset, top + probeInset],
      ['bottom-left', left + probeInset, bottom - probeInset],
      ['bottom-right', right - probeInset, bottom - probeInset],
      ['center', (left + right) / 2, (top + bottom) / 2],
    ];
    for (const [name, x, y] of probes) {
      const hit = document.elementFromPoint(x, y);
      if (!hit || !(el === hit || el.contains(hit))) {
        hidden.push(`${name} covered by ${hit?.tagName.toLowerCase()}.${hit?.className}`);
      }
    }
    const scroller = document.scrollingElement as HTMLElement;
    return {
      viewport,
      panel: { x: box.x, y: box.y, width: box.width, height: box.height },
      mask,
      hidden,
      scroll: {
        x: window.scrollX,
        y: window.scrollY,
        scrollWidth: scroller.scrollWidth,
        scrollHeight: scroller.scrollHeight,
      },
    };
  }, inset);
}

interface Expectation {
  /** A modal: the backdrop must equal the viewport. */
  readonly modal?: boolean;
  /** The panel may be taller than the viewport (it scrolls inside); only its top must be visible. */
  readonly tall?: boolean;
}

/** Asserts the geometry rules on a panel that is already open. */
async function expectCoversViewport(
  panel: Locator,
  label: string,
  options: Expectation = {},
): Promise<void> {
  await expect(panel, `${label} is visible`).toBeVisible();
  // Entrance animations (fade and scale) finish before measuring.
  await panel.page().waitForTimeout(350);
  const g = await measure(panel, CORNER_INSET);
  const { viewport, panel: box } = g;

  expect(box.x, `${label}: left edge inside the viewport`).toBeGreaterThanOrEqual(-TOLERANCE);
  expect(box.y, `${label}: top edge inside the viewport`).toBeGreaterThanOrEqual(-TOLERANCE);
  expect(box.x + box.width, `${label}: right edge inside the viewport`).toBeLessThanOrEqual(
    viewport.width + TOLERANCE,
  );
  if (!options.tall) {
    expect(box.y + box.height, `${label}: bottom edge inside the viewport`).toBeLessThanOrEqual(
      viewport.height + TOLERANCE,
    );
  }
  if (options.modal) {
    expect(g.mask, `${label}: has a fixed mask`).not.toBeNull();
    const mask = g.mask as NonNullable<Geometry['mask']>;
    expect(Math.abs(mask.x), `${label}: mask starts at the left edge`).toBeLessThanOrEqual(
      TOLERANCE,
    );
    expect(Math.abs(mask.y), `${label}: mask starts at the top edge`).toBeLessThanOrEqual(
      TOLERANCE,
    );
    expect(Math.abs(mask.width - viewport.width), `${label}: mask is as wide`).toBeLessThanOrEqual(
      TOLERANCE,
    );
    expect(
      Math.abs(mask.height - viewport.height),
      `${label}: mask is as tall`,
    ).toBeLessThanOrEqual(TOLERANCE);
  }
  expect(g.hidden, `${label}: nothing clips or covers the panel`).toEqual([]);
  expect(g.scroll.y, `${label}: the page did not scroll`).toBe(0);
  expect(g.scroll.x, `${label}: the page did not scroll sideways`).toBe(0);
  expect(
    g.scroll.scrollHeight,
    `${label}: the document is not taller than the viewport`,
  ).toBeLessThanOrEqual(viewport.height + TOLERANCE);
  expect(
    g.scroll.scrollWidth,
    `${label}: the document is not wider than the viewport`,
  ).toBeLessThanOrEqual(viewport.width + TOLERANCE);
}

/** Opens the issue list and waits for the shell. */
async function openShell(page: Page): Promise<void> {
  await page.goto(appPath('/issues'));
  await expect(page.locator('header.top-bar')).toBeVisible();
}

test.describe('los dialogos y paneles cubren la pantalla y no se recortan', () => {
  test('crear incidencia: ventana y selectores dentro de ella', async ({ as }) => {
    const page = await as('owner');
    await openShell(page);
    await page.getByRole('button', { name: 'Crear', exact: true }).click();
    const dialog = page.getByRole('dialog').first();
    await expectCoversViewport(dialog, 'create-issue dialog', { modal: true, tall: true });

    // A select inside the dialog opens its list in the body, above the dialog and inside the viewport.
    const select = dialog.locator('p-select').first();
    await select.click();
    const list = page.locator('.p-select-overlay').last();
    await expectCoversViewport(list, 'select list inside the dialog');
    await page.keyboard.press('Escape');
    await expect(list).toBeHidden();
    await expect(dialog).toBeVisible();
  });

  test('menu de acciones y confirmacion de eliminar incidencia', async ({ as, api, data }) => {
    const { issue } = await projectWithIssue(api, data, 'ovl');
    const page = await as('owner');
    await openIssueView(page, issue.key);
    await page.getByRole('button', { name: 'Más acciones' }).click();
    const menu = page.locator('.p-menu-overlay').last();
    await expectCoversViewport(menu, 'actions menu');
    await page.getByRole('menuitem', { name: 'Eliminar' }).click();
    const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog')).last();
    await expectCoversViewport(confirm, 'confirm modal', { modal: true });
    await confirm.getByRole('button', { name: 'Cancelar' }).click();
    await expect(confirm).toBeHidden();
  });

  test('documentos legales y selector de pais en el registro', async ({ page }) => {
    await page.goto(appPath('/register'));
    await expect(page.getByTestId('register-step-1')).toBeVisible();
    await page.getByRole('button', { name: /^País:/ }).click();
    const panel = page.locator('.phone-input__panel');
    await expectCoversViewport(panel, 'phone country panel');
    expect(await panel.evaluate((el) => !!el.closest('.cdk-overlay-container'))).toBe(true);
    await page.keyboard.press('Escape');
    await expect(panel).toBeHidden();

    await fillRegisterStep1(page, ACCOUNT);
    await page.getByTestId('register-next').click();
    await page.getByTestId('register-step-2').waitFor({ state: 'visible' });
    await page.getByLabel('Correo electrónico').fill(ACCOUNT.email);
    await page.getByLabel(/^Contraseña/).fill(ACCOUNT.password);
    await page.getByLabel(/^Confirmar contraseña/).fill(ACCOUNT.password);
    await page.getByTestId('register-next').click();
    await page.getByTestId('register-step-3').waitFor({ state: 'visible' });
    // The step animation leaves no transform behind (it would trap the fixed mask).
    await page.waitForTimeout(400);
    await page.getByRole('button', { name: 'Política de privacidad', exact: true }).first().click();
    const dialog = page.getByRole('dialog', { name: 'Política de privacidad' });
    await expectCoversViewport(dialog, 'legal document dialog', { modal: true });
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  });

  test('atajos de teclado y paleta de comandos', async ({ as }) => {
    const page = await as('owner');
    await openShell(page);
    await page.locator('body').click({ position: { x: 2, y: 2 } });

    await expect(async () => {
      await page.keyboard.press('?');
      await expect(page.getByRole('dialog', { name: 'Atajos de teclado' })).toBeVisible({
        timeout: 1_500,
      });
    }).toPass();
    await expectCoversViewport(
      page.getByRole('dialog', { name: 'Atajos de teclado' }),
      'shortcuts dialog',
      {
        modal: true,
      },
    );
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog', { name: 'Atajos de teclado' })).toBeHidden();

    await expect(async () => {
      await page.keyboard.press('Control+k');
      await expect(page.getByRole('dialog', { name: 'Paleta de comandos' })).toBeVisible({
        timeout: 1_500,
      });
    }).toPass();
    await expectCoversViewport(
      page.getByRole('dialog', { name: 'Paleta de comandos' }),
      'command palette',
      {
        modal: true,
      },
    );
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog', { name: 'Paleta de comandos' })).toBeHidden();
  });

  test('panel de notificaciones, menu de usuario y ayuda', async ({ as }) => {
    const page = await as('owner');
    await openShell(page);

    await page.getByRole('button', { name: /^Notificaciones/ }).click();
    const bell = page.locator('.p-popover').last();
    await expectCoversViewport(bell, 'notifications panel');
    await page.keyboard.press('Escape');
    await expect(bell).toBeHidden();

    const user = page.getByRole('button', { name: /^Menú de / });
    await user.click();
    const userMenu = page.getByRole('menu', { name: /^Cuenta de / });
    await expectCoversViewport(userMenu, 'user menu');
    expect(await userMenu.evaluate((el) => !!el.closest('.cdk-overlay-container'))).toBe(true);
    await page.keyboard.press('Escape');
    await expect(userMenu).toBeHidden();
    await expect(user).toBeFocused();

    const help = page.getByRole('button', { name: 'Ayuda', exact: true });
    test.skip(await help.isHidden(), 'the help menu is not in the top bar on this viewport');
    await help.click();
    const helpMenu = page.getByRole('menu', { name: 'Ayuda' });
    await expectCoversViewport(helpMenu, 'help menu');
    await page.keyboard.press('Escape');
    await expect(helpMenu).toBeHidden();
  });

  test('selector de organizacion', async ({ as }, testInfo) => {
    const page = await as('owner');
    await openShell(page);

    const switcher = page.getByRole('button', { name: /^Organización actual/ });
    if (await switcher.isVisible()) {
      await switcher.click();
      const menu = page.getByRole('menu', { name: /rganiza/ });
      await expectCoversViewport(menu, 'organization switcher');
      await page.keyboard.press('Escape');
      await expect(menu).toBeHidden();
    } else {
      expect(
        (testInfo.project.use.viewport?.width ?? 0) < 768,
        'the switcher is only missing on phones',
      ).toBe(true);
    }
  });

  test('resultados de la busqueda del encabezado', async ({ as }) => {
    const page = await as('owner');
    await openShell(page);
    const search = page.getByRole('searchbox', { name: /Buscar proyectos/ });
    test.skip(await search.isHidden(), 'the header search is an icon on this viewport');
    await search.fill('inc');
    const results = page.locator('.global-search__panel');
    await expectCoversViewport(results, 'header search results');
    expect(await results.evaluate((el) => !!el.closest('.cdk-overlay-container'))).toBe(true);
    await page.keyboard.press('Escape');
    await expect(results).toBeHidden();
  });

  test('menu de transiciones, dialogo de transicion con selectores y fecha', async ({
    as,
    api,
    data,
  }) => {
    const { owner, project, issue } = await projectWithIssue(api, data, 'ovt');
    const workflow = await readWorkflow(owner, project.id);
    const target = workflow.statuses.find((status) => status.id !== workflow.initial_status_id);
    expect(target, 'a second status to move to').toBeDefined();
    await addTransitionViaApi(owner, project.id, {
      id: 'completar',
      name: 'Completar con datos',
      from: [workflow.initial_status_id],
      to: (target as { id: string }).id,
      validators: [
        { type: 'comment_required' },
        { type: 'resolution_required' },
        { type: 'assignee_set' },
      ],
      required_fields: ['due_date'],
    });

    const page = await as('owner');
    await openIssueView(page, issue.key);
    await page.getByTestId('transition-button').click();
    const menu = page.getByRole('menu', { name: 'Cambiar estado' });
    await expectCoversViewport(menu, 'transition menu');
    expect(await menu.evaluate((el) => !!el.closest('.cdk-overlay-container'))).toBe(true);
    await page.getByTestId('transition-option-completar').click();

    const dialog = page.getByRole('dialog').last();
    await expectCoversViewport(dialog, 'transition dialog', { modal: true, tall: true });

    // Pickers inside the dialog: the resolution select, the user picker and the date field.
    const select = dialog.locator('p-select').first();
    await select.click();
    const list = page.locator('.p-select-overlay').last();
    await expectCoversViewport(list, 'resolution list inside the transition dialog');
    await page.keyboard.press('Escape');
    await expect(list).toBeHidden();
    await expect(dialog).toBeVisible();

    const people = dialog.locator('app-user-picker input').first();
    await people.click();
    const peopleList = page.locator('.user-picker__panel');
    await expectCoversViewport(peopleList, 'user picker list inside the transition dialog');
    expect(await peopleList.evaluate((el) => !!el.closest('.cdk-overlay-container'))).toBe(true);
    await page.keyboard.press('Escape');
    await expect(peopleList).toBeHidden();
    await expect(dialog).toBeVisible();

    const date = dialog.locator('p-datepicker input, app-date-field input').first();
    await date.click();
    const calendar = page.locator('.p-datepicker-panel, .p-datepicker').last();
    await expectCoversViewport(calendar, 'date picker inside the transition dialog');
    await page.keyboard.press('Escape');
    await expect(dialog).toBeVisible();
  });
});
