import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { BacklogApiService, BacklogItem, ProjectApiService, SessionService } from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { NEVER, Observable, of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  PROJECT_ID,
  ensureMatchMedia,
  failWith,
  projectOf,
  projectRoutes,
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

async function renderPage(list: Observable<BacklogItem[]> = of(items)) {
  TestBed.resetTestingModule();
  const backlogApi = {
    list: vi.fn(() => list),
    move: vi.fn((_projectId: string, _issueId: string, _position: number) =>
      of([items[1], items[0]]),
    ),
  };
  const projectApi = { get: vi.fn(() => of(projectOf())) };
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter(projectRoutes(ProjectBacklogPage, 'backlog')),
      { provide: SessionService, useValue: signedInSession() },
      { provide: BacklogApiService, useValue: backlogApi },
      { provide: ProjectApiService, useValue: projectApi },
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
});
