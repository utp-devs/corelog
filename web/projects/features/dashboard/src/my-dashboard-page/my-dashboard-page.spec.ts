import { ComponentFixture, TestBed } from '@angular/core/testing';
import {
  DashboardApiService,
  DashboardStore,
  MyDashboard,
  Project,
  ProjectApiService,
  RECENT_STORAGE_KEY,
} from '@corelog/core';
import { installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of, throwError } from 'rxjs';
import { type MockInstance, afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { MyDashboardPage } from './my-dashboard-page';

const busySummary: MyDashboard = {
  assignedOpen: 4,
  assignedByStatus: { open: 2, in_progress: 2 },
  overdue: 1,
  projectCount: 3,
  activeSprints: [{ id: 's-1', name: 'Sprint 1', projectKey: 'CORE', endDate: '2026-11-30' }],
  recentIssues: [
    {
      id: 'i-1',
      title: 'Revisar exportacion',
      status: 'open',
      priority: 'high',
      projectKey: 'CORE',
      dueDate: '2026-11-25',
    },
  ],
};

const idleSummary: MyDashboard = {
  assignedOpen: 0,
  assignedByStatus: {},
  overdue: 0,
  projectCount: 1,
  activeSprints: [],
  recentIssues: [],
};

const project: Project = {
  id: 'p-1',
  organizationId: 'org-1',
  key: 'CORE',
  name: 'CoreLog',
  description: '',
  type: 'kanban',
  leadId: 'u-1',
  visibility: 'open',
  archivedAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const dashboardApi = { me: vi.fn<() => Observable<MyDashboard>>() };
const projectApi = { list: vi.fn<() => Observable<Project[]>>() };

let restoreViewport: (() => void) | undefined;
let errorSpy: MockInstance<typeof console.error>;
let warnSpy: MockInstance<typeof console.warn>;

// chart.js logs when jsdom has no canvas context. The chart card has its own specs, so that message is ignored here.
const CHART_CONTEXT_MESSAGE = 'Failed to create chart';

function expectNoConsoleOutput(): void {
  const output = [...errorSpy.mock.calls, ...warnSpy.mock.calls].filter(
    (args) => !String(args[0]).startsWith(CHART_CONTEXT_MESSAGE),
  );
  expect(output).toEqual([]);
}

function create(): ComponentFixture<MyDashboardPage> {
  // Stores reset their data when no session is active; run that effect before the page loads its data.
  TestBed.inject(DashboardStore);
  TestBed.flushEffects();
  const fixture = TestBed.createComponent(MyDashboardPage);
  fixture.detectChanges();
  return fixture;
}

function el(fixture: ComponentFixture<MyDashboardPage>): HTMLElement {
  return fixture.nativeElement as HTMLElement;
}

function textOf(fixture: ComponentFixture<MyDashboardPage>): string {
  return el(fixture).textContent ?? '';
}

function byRole(fixture: ComponentFixture<MyDashboardPage>, role: string): HTMLElement | null {
  return el(fixture).querySelector(`[role="${role}"]`);
}

function linkNamed(
  fixture: ComponentFixture<MyDashboardPage>,
  name: string,
): HTMLAnchorElement | undefined {
  return Array.from(el(fixture).querySelectorAll('a')).find(
    (anchor) => anchor.textContent?.trim() === name,
  ) as HTMLAnchorElement | undefined;
}

function list(fixture: ComponentFixture<MyDashboardPage>, label: string): HTMLElement | null {
  return el(fixture).querySelector(`ul[aria-label="${label}"]`);
}

function sectionOf(
  fixture: ComponentFixture<MyDashboardPage>,
  title: string,
): HTMLElement | undefined {
  return Array.from(el(fixture).querySelectorAll('section')).find(
    (section) => section.querySelector('h2')?.textContent?.trim() === title,
  );
}

function configure(): void {
  TestBed.configureTestingModule({
    imports: [MyDashboardPage],
    providers: [
      provideCorelogTesting(),
      { provide: DashboardApiService, useValue: dashboardApi },
      { provide: ProjectApiService, useValue: projectApi },
    ],
  });
}

describe('MyDashboardPage', () => {
  beforeEach(() => {
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    window.localStorage.clear();
    dashboardApi.me.mockReturnValue(of(busySummary));
    projectApi.list.mockReturnValue(of([project]));
    configure();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    restoreViewport?.();
    restoreViewport = undefined;
    TestBed.resetTestingModule();
    vi.clearAllMocks();
  });

  it('shows the personal figures, the active sprints and the urgent issues', () => {
    const fixture = create();

    expect(textOf(fixture)).toContain('Revisar exportacion');
    expect(textOf(fixture)).toContain('Vencidas');
    expect(textOf(fixture)).toMatch(/CORE · Sprint 1/);
    expectNoConsoleOutput();
  });

  it('shows the status breakdown as an accessible data table too', () => {
    const fixture = create();

    const table = el(fixture).querySelector('table[aria-label="Datos: Por estado"]');
    expect(table).not.toBeNull();
    expect(table?.textContent).toContain('En curso');
    expect(table?.textContent).toContain('2');
    expectNoConsoleOutput();
  });

  it('shows the getting-started card instead of an empty page when there is no assigned work', () => {
    dashboardApi.me.mockReturnValue(of(idleSummary));
    const fixture = create();

    expect(sectionOf(fixture, 'Primeros pasos')).toBeDefined();
    expect(linkNamed(fixture, 'Crea tu primer proyecto')?.getAttribute('href')).toBe('/projects');
    expect(textOf(fixture)).toContain('Sin trabajo asignado.');
    // The three steps are separate list items inside the empty-state block, so their copy does not share a box.
    const steps = el(fixture).querySelector('ol[data-empty-state]');
    expect(steps?.querySelectorAll('li')).toHaveLength(3);
    expectNoConsoleOutput();
  });

  it('does not show the getting-started card to users with assigned work', () => {
    const fixture = create();

    expect(sectionOf(fixture, 'Primeros pasos')).toBeUndefined();
    expect(el(fixture).querySelector('ol[data-empty-state]')).toBeNull();
  });

  it('lists the projects with a link to their board', () => {
    dashboardApi.me.mockReturnValue(of(idleSummary));
    const fixture = create();

    expect(linkNamed(fixture, 'CoreLog')?.getAttribute('href')).toBe('/projects/p-1/board');
  });

  it('shows the recently visited items from the local history', () => {
    window.localStorage.setItem(
      RECENT_STORAGE_KEY,
      JSON.stringify([
        {
          kind: 'issue',
          id: 'i-9',
          title: 'Incidencia vista',
          subtitle: 'CORE-9',
          link: '/issues/i-9',
        },
      ]),
    );
    const fixture = create();

    expect(linkNamed(fixture, 'Incidencia vista')?.getAttribute('href')).toBe('/issues/i-9');
    expect(textOf(fixture)).toContain('Incidencia');
    expectNoConsoleOutput();
  });

  it('shows an error message when the summary cannot be loaded', () => {
    dashboardApi.me.mockReturnValue(throwError(() => new Error('network')));
    const fixture = create();

    expect(byRole(fixture, 'alert')?.textContent).toContain('No se pudo cargar tu resumen');
    expectNoConsoleOutput();
  });

  it('shows the skeleton instead of the figures while the summary loads', () => {
    dashboardApi.me.mockReturnValue(NEVER);
    const fixture = create();

    expect(textOf(fixture)).not.toContain('Abiertas asignadas');
    expect(list(fixture, 'Mis proyectos')).toBeNull();
    expect(byRole(fixture, 'alert')).toBeNull();
    expect(el(fixture).querySelector('[role="status"][aria-label="Cargando"]')).not.toBeNull();
    expectNoConsoleOutput();
  });

  it('stacks the panels in one column on mobile', () => {
    restoreViewport = installViewport(390);
    const fixture = create();

    expect(list(fixture, 'Mis proyectos')).not.toBeNull();
    expect(el(fixture).querySelector('.panels')?.getAttribute('data-layout')).toBe('stacked');
    expectNoConsoleOutput();
  });

  it('keeps the panel grid on desktop', () => {
    restoreViewport = installViewport(1440);
    const fixture = create();

    expect(list(fixture, 'Mis proyectos')).not.toBeNull();
    expect(el(fixture).querySelector('.panels')?.getAttribute('data-layout')).toBe('grid');
    expectNoConsoleOutput();
  });
});
