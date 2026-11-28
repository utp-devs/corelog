import type { Locator, Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { orgIdBySlug } from '../../lib/api.ts';
import { appPath } from '../../lib/paths.ts';

interface Box {
  readonly x: number;
  readonly width: number;
}

async function boxes(locator: Locator): Promise<Box[]> {
  const result: Box[] = [];
  for (const item of await locator.all()) {
    const box = await item.boundingBox();
    if (box) result.push({ x: box.x, width: box.width });
  }
  return result;
}

function expectAligned(heads: Box[], columns: Box[], what: string): void {
  expect(heads.length).toBeGreaterThan(0);
  expect(columns.length).toBeGreaterThanOrEqual(heads.length);
  heads.forEach((head, index) => {
    expect(Math.abs(head.x - columns[index].x), `x of ${what} ${index}`).toBeLessThanOrEqual(1);
    expect(
      Math.abs(head.width - columns[index].width),
      `width of ${what} ${index}`,
    ).toBeLessThanOrEqual(1);
  });
}

async function openCalendar(page: Page, key: string, view: string): Promise<void> {
  await page.goto(appPath(`/projects/${key}/calendar`));
  await expect(page.getByTestId('calendar-view')).toBeVisible();
  await page.getByRole('button', { name: view, exact: true }).click();
}

test.describe('alineacion del calendario', () => {
  test('en Semana las cabeceras coinciden con sus columnas y Todo el dia no se parte', async ({
    as,
    api,
    data,
  }, testInfo) => {
    test.skip((testInfo.project.use.viewport?.width ?? 0) < 700, 'phones use the day view');
    const owner = await api('owner');
    const project = await data.createProject('owner', await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');
    await openCalendar(page, project.key, 'Semana');
    const grid = page.getByTestId('calendar-week');
    await expect(grid).toBeVisible();

    const heads = await boxes(grid.locator('.calendar-time-grid__day'));
    expect(heads).toHaveLength(7);
    expectAligned(heads, await boxes(grid.locator('.calendar-time-grid__column')), 'day');

    const label = grid.locator('.calendar-time-grid__gutter--label');
    const lineHeight = await label.evaluate((node) =>
      parseFloat(getComputedStyle(node).lineHeight),
    );
    const labelBox = await label.boundingBox();
    // One text line plus its vertical padding: a wrapped label would be a full line taller.
    expect(labelBox!.height).toBeLessThan(lineHeight * 2 + 8);
    expect(await label.evaluate((node) => node.scrollWidth <= node.clientWidth)).toBe(true);
  });

  test('en Mes los nombres de dia coinciden con las columnas', async ({
    as,
    api,
    data,
  }, testInfo) => {
    test.skip((testInfo.project.use.viewport?.width ?? 0) < 700, 'phones use the day view');
    const owner = await api('owner');
    const project = await data.createProject('owner', await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');
    await openCalendar(page, project.key, 'Mes');
    const month = page.getByTestId('calendar-month');
    await expect(month).toBeVisible();
    const cells = month.locator('.calendar-month__week').first().locator('.calendar-month__day');
    expectAligned(
      await boxes(month.locator('.calendar-month__weekday')),
      await boxes(cells),
      'weekday',
    );
  });

  test('en Dia la cabecera y la columna coinciden', async ({ as, api, data }) => {
    const owner = await api('owner');
    const project = await data.createProject('owner', await orgIdBySlug(owner, 'acme'));
    const page = await as('owner');
    await openCalendar(page, project.key, 'Día');
    const grid = page.getByTestId('calendar-day');
    await expect(grid).toBeVisible();
    expectAligned(
      await boxes(grid.locator('.calendar-time-grid__day')),
      await boxes(grid.locator('.calendar-time-grid__column')),
      'day',
    );
  });
});
