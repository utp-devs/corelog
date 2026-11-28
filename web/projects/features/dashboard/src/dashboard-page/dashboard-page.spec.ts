import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { AuthStore, SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { of } from 'rxjs';

import { DashboardPage } from './dashboard-page';

function dashboardDto(overrides: Record<string, unknown> = {}) {
  return {
    id: 'd-1',
    name: 'Equipo',
    description: 'Panel del equipo',
    layout: '2',
    is_default: false,
    owner: { id: 'u-1', name: 'Ana' },
    organization_id: 'o-1',
    access: 'owner',
    can_edit: true,
    can_manage: true,
    is_favorite: false,
    shared: false,
    gadget_count: 2,
    gadgets: [
      {
        id: 'g-1',
        type: 'text',
        title: 'Aviso',
        config: { markdown: 'Hola' },
        position: 0,
        column_index: 0,
      },
      {
        id: 'g-2',
        type: 'text',
        title: 'Notas',
        config: { markdown: 'Notas' },
        position: 0,
        column_index: 1,
      },
    ],
    shares: [],
    updated_at: '2026-11-27T10:00:00Z',
    ...overrides,
  };
}

async function setup(params: Record<string, string> = { id: 'd-1' }, dto = dashboardDto()) {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: SessionService, useValue: { isAuthenticated: signal(true).asReadonly() } },
      { provide: AuthStore, useValue: { displayName: signal('Ana Torres') } },
      { provide: ActivatedRoute, useValue: { paramMap: of(convertToParamMap(params)) } },
    ],
  });
  const fixture = TestBed.createComponent(DashboardPage);
  const backend = TestBed.inject(HttpTestingController);
  fixture.detectChanges();
  const home = !params['id'];
  backend
    .expectOne(home ? '/api/v1/me/dashboard' : `/api/v1/dashboards/${params['id']}`)
    .flush({ data: dto });
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return { fixture, backend, el: fixture.nativeElement as HTMLElement };
}

function click(el: HTMLElement, selector: string, fixture: ComponentFixture<unknown>): void {
  const target = el.querySelector(selector) as HTMLElement | null;
  if (!target) throw new Error(`missing ${selector}`);
  target.click();
  fixture.detectChanges();
}

const page = (fixture: ComponentFixture<DashboardPage>) =>
  fixture.componentInstance as unknown as {
    session: {
      dirty(): boolean;
      gadgets(): readonly { id: string; title: string; columnIndex: number }[];
    };
    editing(): boolean;
  };

describe('DashboardPage', () => {
  it('shows the gadgets of a dashboard in the columns of its layout', async () => {
    const { el } = await setup();
    expect(el.querySelector('h1')?.textContent).toContain('Equipo');
    const columns = el.querySelectorAll('.dashboard-page__column');
    expect(columns).toHaveLength(2);
    expect(columns[0].textContent).toContain('Aviso');
    expect(columns[1].textContent).toContain('Notas');
    expect(el.querySelector('.dashboard-welcome')).toBeNull();
  });

  it('shows the greeting banner and the personal title on the home dashboard', async () => {
    const { el, backend } = await setup({}, dashboardDto({ is_default: true, name: 'Mi panel' }));
    backend.verify();
    expect(el.querySelector('h1')?.textContent).toContain('Mi resumen');
    expect(el.querySelector('.dashboard-welcome__greeting')?.textContent).toContain('Hola, Ana');
  });

  it('is read-only for a viewer: no edit, no share, and a notice', async () => {
    const { el } = await setup(
      { id: 'd-1' },
      dashboardDto({ access: 'view', can_edit: false, can_manage: false, shares: undefined }),
    );
    expect(el.querySelector('[data-testid="dashboard-edit"]')).toBeNull();
    expect(el.querySelector('[data-testid="dashboard-share"]')).toBeNull();
    expect(el.querySelector('[data-testid="dashboard-readonly"]')?.textContent).toContain('Ana');
    expect(el.querySelector('[data-testid="dashboard-favorite"]')).not.toBeNull();
  });

  it('shows a not available message when the dashboard is hidden', async () => {
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: SessionService, useValue: { isAuthenticated: signal(true).asReadonly() } },
        { provide: AuthStore, useValue: { displayName: signal('Ana') } },
        { provide: ActivatedRoute, useValue: { paramMap: of(convertToParamMap({ id: 'x' })) } },
      ],
    });
    const fixture = TestBed.createComponent(DashboardPage);
    fixture.detectChanges();
    TestBed.inject(HttpTestingController)
      .expectOne('/api/v1/dashboards/x')
      .flush({ error: { message: 'no' } }, { status: 404, statusText: 'Not Found' });
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain(
      'Este panel no está disponible',
    );
  });

  it('adds a gadget through the catalog and saves layout and gadgets in one request', async () => {
    const { fixture, backend, el } = await setup();
    click(el, '[data-testid="dashboard-edit"]', fixture);
    expect(page(fixture).editing()).toBe(true);
    expect(page(fixture).session.dirty()).toBe(false);

    click(el, '[data-testid="dashboard-add-gadget"]', fixture);
    const catalog = document.querySelector('[data-testid="catalog-text"]') as HTMLElement;
    expect(document.querySelectorAll('[data-testid^="catalog-"]')).toHaveLength(13);
    catalog.click();
    fixture.detectChanges();
    (
      document.querySelector(
        '[data-testid="gadget-config-apply"] button, button[data-testid="gadget-config-apply"]',
      ) as HTMLElement
    ).click();
    fixture.detectChanges();

    expect(page(fixture).session.dirty()).toBe(true);
    expect(page(fixture).session.gadgets()).toHaveLength(3);
    expect(el.querySelector('[data-testid="gadget-unsaved"]')).toBeNull();

    click(el, '[data-testid="dashboard-save"]', fixture);
    const request = backend.expectOne('/api/v1/dashboards/d-1/gadgets');
    expect(request.request.method).toBe('PUT');
    const body = request.request.body as {
      layout: string;
      gadgets: Array<{ id?: string; type: string; column_index: number }>;
    };
    expect(body.layout).toBe('2');
    expect(body.gadgets).toHaveLength(3);
    expect(body.gadgets.filter((g) => g.id === undefined)).toHaveLength(1);
    request.flush({ data: dashboardDto() });
    fixture.detectChanges();

    expect(page(fixture).editing()).toBe(false);
    expect(page(fixture).session.dirty()).toBe(false);
  });

  it('reports unsaved edits through dirty and drops them on cancel', async () => {
    const { fixture, el } = await setup();
    click(el, '[data-testid="dashboard-edit"]', fixture);
    expect(fixture.componentInstance.dirty()).toBe(false);

    (
      fixture.componentInstance as unknown as {
        removeGadget(id: string, title: string): void;
      }
    ).removeGadget('g-1', 'Aviso');
    fixture.detectChanges();
    expect(fixture.componentInstance.dirty()).toBe(true);
    expect(el.querySelectorAll('.dashboard-page__gadget')).toHaveLength(1);
  });

  it('changes the layout while editing', async () => {
    const { fixture, el } = await setup();
    click(el, '[data-testid="dashboard-edit"]', fixture);
    click(el, '[data-testid="dashboard-layout-3"]', fixture);
    expect(el.querySelectorAll('.dashboard-page__column')).toHaveLength(3);
    expect(fixture.componentInstance.dirty()).toBe(true);
  });

  it('moves a gadget to another column from the keyboard menu action', async () => {
    const { fixture, el } = await setup();
    click(el, '[data-testid="dashboard-edit"]', fixture);
    (
      fixture.componentInstance as unknown as {
        moveToColumn(id: string, title: string, column: number): void;
      }
    ).moveToColumn('g-1', 'Aviso', 1);
    fixture.detectChanges();
    const columns = el.querySelectorAll('.dashboard-page__column');
    expect(columns[0].querySelectorAll('.dashboard-page__gadget')).toHaveLength(0);
    expect(columns[1].querySelectorAll('.dashboard-page__gadget')).toHaveLength(2);
  });
});
