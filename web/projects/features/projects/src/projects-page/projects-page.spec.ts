import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { FavoritesApiService, OrganizationMember, Project } from '@corelog/core';
import { guardConsole, provideCorelogTesting } from '@corelog/core/testing';
import { Observable, NEVER, of, throwError } from 'rxjs';
import { Mock, afterEach, describe, expect, it, vi } from 'vitest';

import { projectOf, projectTestEnvironment } from '../testing/projects-test-kit';
import { ProjectsPage } from './projects-page';

function orgMember(userId: string, name: string): OrganizationMember {
  return { userId, name, displayName: name } as unknown as OrganizationMember;
}

function page(items: Project[], total = items.length) {
  return of({ items, meta: { total, page: 1, pageSize: 20 } });
}

const projects: Project[] = [
  projectOf({ id: 'p-1', key: 'CORE', name: 'CoreLog', template: 'scrum' }),
  projectOf({
    id: 'p-2',
    key: 'OLD',
    name: 'Heredado',
    template: 'kanban',
    archivedAt: '2026-02-01T00:00:00Z',
  }),
];

describe('ProjectsPage', () => {
  const consoleGuard = guardConsole();

  afterEach(() => {
    TestBed.resetTestingModule();
    localStorage.removeItem('corelog.projects.view');
  });

  async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
    for (let pass = 0; pass < 3; pass += 1) {
      fixture.detectChanges();
      await Promise.resolve();
    }
  }

  async function render(
    options: Parameters<typeof projectTestEnvironment>[0] = {},
    favorites: Partial<Record<'list' | 'star' | 'unstar', () => Observable<unknown>>> = {},
  ): Promise<{
    fixture: ComponentFixture<ProjectsPage>;
    env: ReturnType<typeof projectTestEnvironment>;
    favorites: Record<string, Mock>;
    root: HTMLElement;
  }> {
    const env = projectTestEnvironment({
      organization: { role: 'member', policy: 'members' },
      ...options,
    });
    const favoriteApi = {
      list: vi.fn(favorites.list ?? (() => of([]))),
      star: vi.fn(favorites.star ?? (() => of(undefined))),
      unstar: vi.fn(favorites.unstar ?? (() => of(undefined))),
      recent: vi.fn(() => of([])),
    };
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideRouter([]),
        ...env.providers,
        { provide: FavoritesApiService, useValue: favoriteApi },
      ],
    });
    const fixture = TestBed.createComponent(ProjectsPage);
    fixture.detectChanges();
    await settle(fixture);
    return { fixture, env, favorites: favoriteApi, root: fixture.nativeElement as HTMLElement };
  }

  function byTestId(root: HTMLElement, id: string): HTMLElement | null {
    return root.querySelector<HTMLElement>(`[data-testid="${id}"]`);
  }

  it('lists the projects of the server page as a table by default', async () => {
    const { root, env } = await render({
      projectApi: { listPage: () => page(projects) },
      orgApi: { listMembers: () => of([orgMember('u1', 'Ana Pérez')]) },
    });

    expect(env.projectApi['listPage']).toHaveBeenCalledWith(
      expect.objectContaining({ archived: 'exclude', sort: 'name', desc: false, page: 1 }),
    );
    expect(byTestId(root, 'project-row-CORE')?.textContent).toContain('CoreLog');
    expect(root.textContent).toContain('Ana Pérez');
    expect(root.textContent).toContain('Archivado');
    consoleGuard.expectNoConsoleOutput();
  });

  it('switches to cards and remembers the choice', async () => {
    const { fixture, root } = await render({ projectApi: { listPage: () => page(projects) } });

    root.querySelector<HTMLButtonElement>('button[aria-label="Vista de tarjetas"]')?.click();
    await settle(fixture);

    expect(byTestId(root, 'project-card-CORE')).not.toBeNull();
    expect(localStorage.getItem('corelog.projects.view')).toBe('cards');
  });

  it('sends the search text and goes back to the first page', async () => {
    const { fixture, env, root } = await render({
      projectApi: { listPage: () => page(projects, 60) },
    });
    const input = root.querySelector<HTMLInputElement>('input[aria-label="Buscar proyectos"]');
    vi.useFakeTimers();
    try {
      input!.value = 'core';
      input!.dispatchEvent(new Event('input'));
      await vi.advanceTimersByTimeAsync(400);
    } finally {
      vi.useRealTimers();
    }
    await settle(fixture);

    expect(env.projectApi['listPage']).toHaveBeenLastCalledWith(
      expect.objectContaining({ q: 'core', page: 1 }),
    );
  });

  it('filters by starred projects through the toggle', async () => {
    const { fixture, env, root } = await render({
      projectApi: { listPage: () => page(projects) },
    });

    byTestId(root, 'projects-starred-filter')?.querySelector('button')?.click();
    (byTestId(root, 'projects-starred-filter') as HTMLButtonElement | null)?.click();
    await settle(fixture);

    expect(env.projectApi['listPage']).toHaveBeenLastCalledWith(
      expect.objectContaining({ starred: true }),
    );
  });

  it('stars a project at once and tells when it fails', async () => {
    const { fixture, favorites, root } = await render(
      { projectApi: { listPage: () => page(projects) } },
      { star: () => throwError(() => ({ status: 500, code: 'x', message: 'x', details: [] })) },
    );

    root
      .querySelector<HTMLButtonElement>('button[aria-label="Marcar como favorito CoreLog"]')
      ?.click();
    await settle(fixture);

    expect(favorites['star']).toHaveBeenCalledWith('project', 'p-1');
    // The star is undone because the request failed.
    expect(root.querySelector('button[aria-label="Marcar como favorito CoreLog"]')).not.toBeNull();
  });

  it('shows the create button to members when the policy allows it', async () => {
    const { root } = await render({ projectApi: { listPage: () => page(projects) } });
    expect(byTestId(root, 'projects-create')).not.toBeNull();
  });

  it('hides the create button for guests and when only admins may create', async () => {
    const guest = await render({
      organization: { role: 'guest', policy: 'members' },
      role: 'user',
      projectApi: { listPage: () => page(projects) },
    });
    expect(byTestId(guest.root, 'projects-create')).toBeNull();
    TestBed.resetTestingModule();

    const closed = await render({
      organization: { role: 'member', policy: 'admins' },
      role: 'user',
      projectApi: { listPage: () => page(projects) },
    });
    expect(byTestId(closed.root, 'projects-create')).toBeNull();
  });

  it('does not read the member directory for guests and reads it silently for others', async () => {
    const guest = await render({
      organization: { role: 'guest', policy: 'members' },
      role: 'user',
      projectApi: { listPage: () => page(projects) },
      orgApi: { listMembers: () => of([]) },
    });
    expect(guest.env.orgApi['listMembers']).not.toHaveBeenCalled();
    TestBed.resetTestingModule();

    const member = await render({
      projectApi: { listPage: () => page(projects) },
      orgApi: { listMembers: () => of([]) },
    });
    expect(member.env.orgApi['listMembers']).toHaveBeenCalledWith(expect.anything(), {
      silent: true,
    });
  });

  it('draws a cover photograph over each card, chosen from the project color or key', async () => {
    const { fixture, root } = await render({ projectApi: { listPage: () => page(projects) } });

    root.querySelector<HTMLButtonElement>('button[aria-label="Vista de tarjetas"]')?.click();
    await settle(fixture);

    const card = byTestId(root, 'project-card-CORE');
    const cover = card?.querySelector('.projects-page__cover img');
    expect(cover?.getAttribute('src')).toContain('/images/covers/cover-0');
    expect(cover?.getAttribute('alt')).toBe('');
    expect(card?.querySelector('.projects-page__cover app-project-avatar')).not.toBeNull();
  });

  it('shows the empty projects photograph when the organization has no projects', async () => {
    const { root } = await render({ projectApi: { listPage: () => page([]) } });

    expect(root.querySelector('img[src="/images/states/empty-projects.webp"]')).not.toBeNull();
  });

  it('teases the templates when the organization has no projects', async () => {
    const { root } = await render({ projectApi: { listPage: () => page([]) } });

    expect(root.textContent).toContain('Aún no hay proyectos');
    expect(root.textContent).toContain('Seguimiento de errores');
    expect(byTestId(root, 'projects-empty-create')).not.toBeNull();
  });

  it('explains an empty search and clears the filters', async () => {
    const { fixture, env, root } = await render({
      projectApi: {
        listPage: vi.fn((filters: { q?: string }) => (filters.q ? page([]) : page(projects))),
      },
    });
    const input = root.querySelector<HTMLInputElement>('input[aria-label="Buscar proyectos"]');
    vi.useFakeTimers();
    try {
      input!.value = 'zzz';
      input!.dispatchEvent(new Event('input'));
      await vi.advanceTimersByTimeAsync(400);
    } finally {
      vi.useRealTimers();
    }
    await settle(fixture);
    expect(root.textContent).toContain('Ningún proyecto coincide');

    byTestId(root, 'projects-clear-filters')?.querySelector('button')?.click();
    (byTestId(root, 'projects-clear-filters') as HTMLElement | null)?.click();
    await settle(fixture);

    expect(env.projectApi['listPage']).toHaveBeenLastCalledWith(
      expect.not.objectContaining({ q: expect.anything() }),
    );
    expect(byTestId(root, 'project-row-CORE')).not.toBeNull();
  });

  it('shows the error state with a retry that reads again', async () => {
    const listPage = vi
      .fn()
      .mockReturnValueOnce(
        throwError(() => ({ status: 500, code: 'http.500', message: 'Falló', details: [] })),
      )
      .mockReturnValue(page(projects));
    const { fixture, root } = await render({ projectApi: { listPage } });

    expect(root.textContent).toContain('Falló');
    const retry = Array.from(root.querySelectorAll('button')).find((button) =>
      /Reintentar/.test(button.textContent ?? ''),
    );
    retry?.click();
    await settle(fixture);

    expect(listPage).toHaveBeenCalledTimes(2);
    expect(byTestId(root, 'project-row-CORE')).not.toBeNull();
  });

  it('shows the forbidden page when the server refuses the list', async () => {
    const { root } = await render({
      projectApi: {
        listPage: () =>
          throwError(() => ({ status: 403, code: 'http.403', message: 'no', details: [] })),
      },
    });
    expect(root.querySelector('app-error-page')).not.toBeNull();
  });

  it('does not render rows while the first page is pending', async () => {
    const { root } = await render({ projectApi: { listPage: () => NEVER } });
    expect(byTestId(root, 'project-row-CORE')).toBeNull();
  });
});
