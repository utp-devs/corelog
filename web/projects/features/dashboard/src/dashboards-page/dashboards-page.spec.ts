import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { vi } from 'vitest';

import { DashboardsPage } from './dashboards-page';

function dto(id: string, overrides: Record<string, unknown> = {}) {
  return {
    id,
    name: `Panel ${id}`,
    description: '',
    layout: '2',
    is_default: false,
    owner: { id: 'u-1', name: 'Ana' },
    organization_id: 'o-1',
    access: 'owner',
    can_edit: true,
    can_manage: true,
    is_favorite: false,
    shared: false,
    gadget_count: 3,
    updated_at: '2026-11-27T10:00:00Z',
    ...overrides,
  };
}

async function setup(
  items = [dto('d-1', { is_default: true }), dto('d-2', { is_favorite: true })],
) {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: SessionService, useValue: { isAuthenticated: signal(true).asReadonly() } },
    ],
  });
  const fixture = TestBed.createComponent(DashboardsPage);
  const backend = TestBed.inject(HttpTestingController);
  fixture.detectChanges();
  await fixture.whenStable();
  backend
    .expectOne((r) => r.url === '/api/v1/dashboards')
    .flush({ data: items, meta: { page: 1, page_size: 100, total: items.length } });
  fixture.detectChanges();
  return { fixture, backend, el: fixture.nativeElement as HTMLElement };
}

describe('DashboardsPage', () => {
  it('lists the dashboards with their badges and gadget count', async () => {
    const { el } = await setup();
    const rows = el.querySelectorAll('[data-testid^="dashboard-row-"]');
    expect(rows).toHaveLength(2);
    expect(rows[0].textContent).toContain('Principal');
    expect(rows[0].textContent).toContain('3 gadgets');
    expect(rows[0].querySelector('a')?.getAttribute('href')).toBe('/dashboards/d-1');
  });

  it('marks a read-only dashboard shared by someone else', async () => {
    const { el } = await setup([
      dto('d-3', { access: 'view', can_edit: false, can_manage: false, shared: true }),
    ]);
    expect(el.textContent).toContain('Solo lectura');
    expect(el.textContent).toContain('Compartido');
  });

  it('shows only favorites in the favorites tab', async () => {
    const { fixture, el } = await setup();
    (el.querySelector('[data-testid="dashboards-tab-favorites"]') as HTMLElement).click();
    fixture.detectChanges();
    const rows = el.querySelectorAll('[data-testid^="dashboard-row-"]');
    expect(rows).toHaveLength(1);
    expect(rows[0].getAttribute('data-testid')).toBe('dashboard-row-d-2');
  });

  it('asks the API for the chosen scope', async () => {
    const { fixture, backend, el } = await setup();
    (el.querySelector('[data-testid="dashboards-tab-shared"]') as HTMLElement).click();
    fixture.detectChanges();
    await fixture.whenStable();
    const request = backend.expectOne((r) => r.url === '/api/v1/dashboards');
    expect(request.request.params.get('scope')).toBe('shared');
    request.flush({ data: [], meta: { page: 1, page_size: 100, total: 0 } });
    fixture.detectChanges();
    expect(el.textContent).toContain('No hay paneles que coincidan');
  });

  it('stars a dashboard', async () => {
    const { fixture, backend, el } = await setup();
    (el.querySelector('[aria-label="Marcar como favorito Panel d-1"]') as HTMLElement).click();
    const star = backend.expectOne('/api/v1/favorites/dashboard/d-1');
    expect(star.request.method).toBe('PUT');
    star.flush(null);
    fixture.detectChanges();
    expect(el.querySelector('[aria-label="Quitar de favoritos Panel d-1"]')).not.toBeNull();
  });

  it('creates a dashboard from the dialog and opens it', async () => {
    const { fixture, backend, el } = await setup();
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    (el.querySelector('[data-testid="dashboards-create"]') as HTMLElement).click();
    fixture.detectChanges();
    const name = document.querySelector('[data-testid="dashboard-name"]') as HTMLInputElement;
    name.value = 'Mi nuevo panel';
    name.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    (document.querySelector('[data-testid="dashboard-form-confirm"]') as HTMLElement)
      .querySelector('button')
      ?.click();
    const request = backend.expectOne('/api/v1/dashboards');
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toMatchObject({ name: 'Mi nuevo panel', layout: '2' });
    request.flush({ data: dto('d-9', { name: 'Mi nuevo panel' }) });
    expect(navigate).toHaveBeenCalledWith(['/dashboards', 'd-9']);
  });

  it('shows a friendly empty state without dashboards', async () => {
    const { el } = await setup([]);
    expect(el.textContent).toContain('Aún no tienes paneles');
  });
});
