import { APP_BASE_HREF } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { Provider } from '@angular/core';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  AuthStore,
  BacklogApiService,
  BacklogItem,
  ProjectApiService,
  ProjectMember,
  SessionService,
} from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  PROJECT_ID,
  ME_ID,
  ensureMatchMedia,
  failWith,
  projectOf,
  projectRoutes,
  signedInAuth,
  signedInSession,
} from '../testing/planning-test-kit';
import { ProjectBacklogPage } from './project-backlog-page';

ensureMatchMedia();

const backlogPath = `/projects/${PROJECT_ID}/backlog`;

const items: BacklogItem[] = [
  {
    id: 'issue-1',
    projectId: PROJECT_ID,
    summary: 'Primera',
    type: 'task',
    status: 'open',
    priority: 'high',
    rank: '000001',
  },
  {
    id: 'issue-2',
    projectId: PROJECT_ID,
    summary: 'Segunda',
    type: 'bug',
    status: 'in_progress',
    priority: 'low',
    rank: '000002',
  },
];

async function settle(harness: RouterTestingHarness): Promise<void> {
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
}

async function renderPage(
  list: Observable<BacklogItem[]> = of(items),
  extraProviders: Provider[] = [],
  members: ProjectMember[] = [],
) {
  TestBed.resetTestingModule();
  const backlogApi = {
    list: vi.fn(() => list),
    move: vi.fn((_projectId: string, _issueId: string, _position: number) =>
      of([items[1], items[0]]),
    ),
  };
  const projectApi = {
    get: vi.fn(() => of(projectOf())),
    listMembers: vi.fn(() => of(members)),
  };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter(projectRoutes(ProjectBacklogPage, 'backlog')),
      { provide: SessionService, useValue: signedInSession() },
      { provide: BacklogApiService, useValue: backlogApi },
      { provide: ProjectApiService, useValue: projectApi },
      { provide: AuthStore, useValue: signedInAuth() },
      ...extraProviders,
    ],
  });
  const harness = await RouterTestingHarness.create(backlogPath);
  await settle(harness);
  return { backlogApi, harness };
}

function buttonNamed(label: string): HTMLButtonElement | null {
  return document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
}

function bodyText(): string {
  return document.body.textContent ?? '';
}

function alertText(): string {
  return document.querySelector('[role="alert"]')?.textContent ?? '';
}

describe('ProjectBacklogPage', () => {
  const consoleGuard = guardConsole();
  let restoreViewport: (() => void) | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    restoreViewport?.();
    restoreViewport = undefined;
    TestBed.resetTestingModule();
    vi.restoreAllMocks();
  });

  it('renders the ordered rows and an empty state inside the table when there are no issues', async () => {
    const { harness } = await renderPage(of([]));

    expect(bodyText()).toContain('El backlog está vacío');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows the 403 page when the backlog is forbidden', async () => {
    const { harness } = await renderPage(failWith(403));

    expect(bodyText()).toContain('No tienes acceso a esta página');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('lists the issues in rank order with their position', async () => {
    const { harness } = await renderPage();

    expect(bodyText()).toContain('Primera');
    expect(bodyText()).toContain('Segunda');
    expect(buttonNamed('Subir Primera')?.disabled).toBe(true);
    expect(buttonNamed('Bajar Segunda')?.disabled).toBe(true);
    harness.fixture.destroy();
  });

  it('shows type and status in Spanish with the short key', async () => {
    const { harness } = await renderPage();

    expect(bodyText()).toContain('Tarea');
    expect(bodyText()).toContain('Error');
    expect(bodyText()).toContain('Abierto');
    expect(bodyText()).toContain('En curso');
    expect(bodyText()).not.toContain('Abierta');
    harness.fixture.destroy();
  });

  it('gives the arrow buttons a visible target and a start/end menu', async () => {
    const { backlogApi, harness } = await renderPage();

    expect(document.querySelector('.project-backlog__arrow')).not.toBeNull();
    buttonNamed('Más opciones de orden de Segunda')?.click();
    harness.fixture.detectChanges();
    const toStart = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(
      (el) => el.textContent?.includes('Mover al inicio'),
    );
    toStart?.click();
    await settle(harness);

    expect(backlogApi.move).toHaveBeenCalledWith(PROJECT_ID, 'issue-2', 0);
    harness.fixture.destroy();
  });

  it('hides the reorder controls from viewers', async () => {
    const { harness } = await renderPage(
      of(items),
      [],
      [{ projectId: PROJECT_ID, userId: ME_ID, role: 'viewer', createdAt: '2026-01-01T00:00:00Z' }],
    );

    expect(bodyText()).toContain('Primera');
    expect(buttonNamed('Bajar Primera')).toBeNull();
    expect(bodyText()).toContain('Solo lectura');
    harness.fixture.destroy();
  });

  it('moves an issue one position down and shows the reordered list', async () => {
    const { backlogApi, harness } = await renderPage();

    buttonNamed('Bajar Primera')?.click();
    await settle(harness);

    expect(backlogApi.move).toHaveBeenCalledWith(PROJECT_ID, 'issue-1', 1);
    expect(buttonNamed('Subir Primera')?.disabled).toBe(false);
    harness.fixture.destroy();
  });

  it('reports a forbidden reorder in Spanish', async () => {
    const { backlogApi, harness } = await renderPage();
    backlogApi.move.mockReturnValue(failWith(403));

    buttonNamed('Bajar Primera')?.click();
    await settle(harness);

    expect(alertText()).toContain('No tienes permiso para ordenar el backlog');
    harness.fixture.destroy();
  });

  it('shows the table skeleton with busy feedback while the backlog loads', async () => {
    const { harness } = await renderPage(NEVER);

    await vi.waitFor(() => {
      const loading = document.querySelector('[role="status"][aria-label="Cargando"]');
      expect(loading?.getAttribute('aria-busy')).toBe('true');
    });
    expect(bodyText()).not.toContain('El backlog está vacío');
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('shows an error inside the table with retry when the backlog fails for another reason', async () => {
    const { backlogApi, harness } = await renderPage(failWith(500));

    expect(alertText()).toContain('No se pudo cargar el backlog');
    backlogApi.list.mockReturnValue(of(items));
    Array.from(document.querySelectorAll('button'))
      .find((button) => button.textContent?.trim() === 'Reintentar')
      ?.click();
    await settle(harness);

    expect(bodyText()).toContain('Primera');
    expect(backlogApi.list).toHaveBeenCalledTimes(2);
    consoleGuard.expectNoConsoleOutput();
    harness.fixture.destroy();
  });

  it('renders a table on desktop and issue cards with move buttons on mobile', async () => {
    restoreViewport = installViewport(1440);
    const desktop = await renderPage();

    expect(document.querySelectorAll('table').length).toBeGreaterThan(0);
    desktop.harness.fixture.destroy();

    restoreViewport();
    restoreViewport = installViewport(390);
    const mobile = await renderPage();

    expect(bodyText()).toContain('Primera');
    expect(document.querySelector('table')).toBeNull();
    const cards = document.querySelectorAll('li');
    expect(cards.length).toBe(2);
    expect(cards[0]?.contains(buttonNamed('Bajar Primera'))).toBe(true);
    consoleGuard.expectNoConsoleOutput();
    mobile.harness.fixture.destroy();
  });

  it('links each row summary to the issue under the workspace base without repeating it', async () => {
    const { harness } = await renderPage(of(items), [
      { provide: APP_BASE_HREF, useValue: '/workspace/' },
    ]);
    const hrefs = Array.from(
      document.querySelectorAll<HTMLAnchorElement>('a.project-backlog__summary'),
    ).map((link) => link.getAttribute('href'));
    expect(hrefs).toContain('/workspace/issues/issue-1');
    expect(hrefs.some((href) => href?.includes('/workspace/workspace/'))).toBe(false);
    harness.fixture.destroy();
  });
});
