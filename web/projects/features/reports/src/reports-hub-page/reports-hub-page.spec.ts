import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { NEVER, throwError } from 'rxjs';
import { afterEach, describe, expect, it } from 'vitest';

import { reportsTestEnvironment, type ReportsTestOptions } from '../testing/reports-test-kit';
import { ReportsHubPage } from './reports-hub-page';

async function open(options: ReportsTestOptions = {}): Promise<ComponentFixture<ReportsHubPage>> {
  const env = reportsTestEnvironment(options);
  TestBed.configureTestingModule({ providers: [provideRouter([]), ...env.providers] });
  const fixture = TestBed.createComponent(ReportsHubPage);
  for (let pass = 0; pass < 3; pass += 1) {
    fixture.detectChanges();
    await Promise.resolve();
  }
  return fixture;
}

function el(fixture: ComponentFixture<ReportsHubPage>): HTMLElement {
  return fixture.nativeElement as HTMLElement;
}

describe('ReportsHubPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the reports of the catalog grouped by theme, each linking to its page', async () => {
    const fixture = await open();
    const titles = Array.from(el(fixture).querySelectorAll('.reports-hub__group-title')).map(
      (node) => node.textContent?.trim(),
    );
    expect(titles).toEqual(['Ágil', 'Tiempo y flujo', 'Versiones']);
    const card = el(fixture).querySelector<HTMLAnchorElement>(
      '[data-testid="report-card-burndown"]',
    );
    expect(card?.getAttribute('href')).toBe('/projects/SCRUM/reports/burndown');
    expect(card?.textContent).toContain('Trabajo restante frente a la línea ideal.');
    expect(card?.textContent).toContain('Abrir');
    // Reports the catalog does not list are not shown.
    expect(el(fixture).querySelector('[data-testid="report-card-burnup"]')).toBeNull();
  });

  it('shows a skeleton while the catalog loads', async () => {
    const fixture = await open({ list: () => NEVER });
    expect(el(fixture).querySelector('app-loading-skeleton')).not.toBeNull();
    expect(el(fixture).querySelector('[data-testid="reports-hub"]')).toBeNull();
  });

  it('shows the forbidden page without the view_reports permission', async () => {
    const fixture = await open({ canViewReports: false });
    expect(el(fixture).querySelector('app-error-page')).not.toBeNull();
    expect(el(fixture).querySelector('[data-testid="reports-hub"]')).toBeNull();
  });

  it('shows an error with retry when the catalog fails', async () => {
    const fixture = await open({
      list: () => throwError(() => ({ status: 500, code: 'http.500', message: 'x', details: [] })),
    });
    expect(el(fixture).querySelector('app-error-state')).not.toBeNull();
  });
});
