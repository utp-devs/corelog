import type { Page } from '@playwright/test';

import { expect, test } from '../../fixtures/test.ts';
import { appPath } from '../../lib/paths.ts';
import { fillRegisterStep1, fillRegisterStep2 } from '../../lib/register.ts';

const DATA = {
  firstName: 'Ana',
  lastName: 'Torres',
  phone: '+51 912 345 678',
  email: 'ana.altura@corelog.test',
  password: 'Secreta-12345',
};

const VIEWPORTS = [
  { width: 1920, height: 1080 },
  { width: 1440, height: 900 },
  { width: 1280, height: 720 },
  { width: 820, height: 1180 },
  { width: 390, height: 844 },
] as const;

interface Size {
  readonly width: number;
  readonly height: number;
}

interface Metrics {
  readonly viewport: Size;
  readonly page: Size;
  readonly hero: Size | null;
  readonly panel: Size;
  readonly panelIsChild: boolean;
  readonly heroIsChild: boolean;
  readonly shell: number;
  readonly scrollHeight: number;
  readonly scrollWidth: number;
  readonly pageScroll: number;
}

/** Sizes of the page and its two panels, and whether the panels are direct children of the page. */
async function measure(page: Page): Promise<Metrics> {
  return page.evaluate(() => {
    const pageEl = document.querySelector('.auth-layout__page') as HTMLElement;
    const hero = document.querySelector('.auth-layout__hero') as HTMLElement | null;
    const panel = document.querySelector('.auth-layout__panel') as HTMLElement;
    const size = (el: HTMLElement): { width: number; height: number } => {
      const box = el.getBoundingClientRect();
      return { width: box.width, height: box.height };
    };
    const scroller = document.scrollingElement as HTMLElement;
    return {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      page: size(pageEl),
      hero: hero ? size(hero) : null,
      panel: size(panel),
      panelIsChild: panel.parentElement === pageEl,
      heroIsChild: hero ? hero.parentElement === pageEl : true,
      shell: document.querySelectorAll('.auth-layout__shell').length,
      scrollHeight: scroller.scrollHeight,
      scrollWidth: scroller.scrollWidth,
      pageScroll: pageEl.scrollTop + pageEl.scrollLeft,
    };
  });
}

/** The page is exactly the viewport, split edge to edge by its two direct children, and never scrolls. */
async function expectFullViewport(page: Page): Promise<void> {
  const m = await measure(page);
  expect(m.shell).toBe(0);
  expect(m.panelIsChild).toBe(true);
  expect(m.heroIsChild).toBe(true);
  expect(Math.abs(m.page.height - m.viewport.height)).toBeLessThanOrEqual(1);
  expect(Math.abs(m.page.width - m.viewport.width)).toBeLessThanOrEqual(1);
  expect(m.scrollHeight).toBeLessThanOrEqual(m.viewport.height);
  expect(m.scrollWidth).toBeLessThanOrEqual(m.viewport.width);
  // Focusing a field or opening a panel must never scroll the clipped page itself.
  expect(m.pageScroll).toBe(0);
  if (m.hero) {
    if (m.viewport.width > 1024) {
      // Two columns edge to edge.
      expect(Math.abs(m.hero.width + m.panel.width - m.viewport.width)).toBeLessThanOrEqual(1);
      expect(Math.abs(m.hero.height - m.viewport.height)).toBeLessThanOrEqual(1);
      expect(Math.abs(m.panel.height - m.viewport.height)).toBeLessThanOrEqual(1);
    } else {
      // Hero strip over the form.
      expect(Math.abs(m.hero.width - m.viewport.width)).toBeLessThanOrEqual(1);
      expect(Math.abs(m.panel.width - m.viewport.width)).toBeLessThanOrEqual(1);
      expect(Math.abs(m.hero.height + m.panel.height - m.viewport.height)).toBeLessThanOrEqual(1);
    }
  } else {
    expect(Math.abs(m.panel.width - m.viewport.width)).toBeLessThanOrEqual(1);
  }
}

test.describe('estructura de las pantallas de acceso', () => {
  test('el registro llena el viewport en cada paso, con y sin errores', async ({ page }) => {
    await page.goto(appPath('/login'));
    await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
    await expectFullViewport(page);

    await page.goto(appPath('/register'));
    await expect(page.getByTestId('register-step-1')).toBeVisible();
    await expectFullViewport(page);

    // Errors of the first step.
    await page.getByTestId('register-next').click();
    await expect(page.getByText('Escribe tus nombres')).toBeVisible();
    await expectFullViewport(page);

    await fillRegisterStep1(page, DATA);
    await page.getByTestId('register-next').click();
    await expect(page.getByTestId('register-step-2')).toBeVisible();
    await expectFullViewport(page);

    // Errors of the second step, then a long mismatch message.
    await page.getByTestId('register-next').click();
    await expect(page.getByText('Confirma tu contraseña')).toBeVisible();
    await expectFullViewport(page);
    await fillRegisterStep2(page, { ...DATA, confirmation: 'otra' });
    await expect(page.getByText('Las contraseñas no coinciden')).toBeVisible();
    await expectFullViewport(page);

    await page.getByLabel(/^Confirmar contraseña/).fill(DATA.password);
    await page.getByTestId('register-next').click();
    await expect(page.getByTestId('register-step-3')).toBeVisible();
    await expectFullViewport(page);
  });

  test('login, olvido de contraseña y registro comparten la estructura en todos los viewports', async ({
    page,
  }) => {
    for (const viewport of VIEWPORTS) {
      await page.setViewportSize(viewport);
      for (const path of ['/login', '/forgot-password', '/register']) {
        await page.goto(appPath(path));
        await expect(page.locator('.auth-layout__panel h1')).toBeVisible();
        await expectFullViewport(page);
      }
    }
  });
});
