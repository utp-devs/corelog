import { ComponentFixture, TestBed } from '@angular/core/testing';
import {
  OrgRole,
  OrgWhoCan,
  NotificationService,
  OrganizationStore,
  TeamStore,
} from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  apiErrorOf,
  fakeOrganizationStore,
  fakeTeamStore,
  FakeTeamStore,
  teamOf,
} from '../testing/team-test-kit';
import { TeamsPage } from './teams-page';

interface Setup {
  fixture: ComponentFixture<TeamsPage>;
  el: HTMLElement;
  store: FakeTeamStore;
  notify: { success: ReturnType<typeof vi.fn>; error: ReturnType<typeof vi.fn> };
}

function setup(role: OrgRole | null = 'admin', policy: OrgWhoCan = 'admins'): Setup {
  const store = fakeTeamStore();
  const organizations = fakeOrganizationStore(role, policy);
  const notify = { success: vi.fn(), error: vi.fn(), info: vi.fn() };
  TestBed.configureTestingModule({
    imports: [TeamsPage],
    providers: [
      provideCorelogTesting(),
      { provide: TeamStore, useValue: store },
      { provide: OrganizationStore, useValue: organizations },
      { provide: NotificationService, useValue: notify },
    ],
  });
  const fixture = TestBed.createComponent(TeamsPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, store, notify };
}

/** Lets debounced search and async submits settle, then re-renders. */
async function settle(fixture: ComponentFixture<TeamsPage>, check: () => void): Promise<void> {
  await vi.waitFor(() => {
    fixture.detectChanges();
    check();
  });
}

function buttonByText(root: ParentNode, text: string): HTMLButtonElement | undefined {
  return Array.from(root.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === text,
  );
}

function createButton(el: HTMLElement): HTMLButtonElement | undefined {
  return buttonByText(el, 'Crear equipo');
}

function dialog(): HTMLElement {
  const found = document.querySelector<HTMLElement>('[role="dialog"]');
  if (!found) throw new Error('dialog not open');
  return found;
}

function typeInto(input: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

function cards(el: HTMLElement): HTMLAnchorElement[] {
  return Array.from(el.querySelectorAll<HTMLAnchorElement>('[data-testid="team-card"]'));
}

describe('TeamsPage', () => {
  const consoleGuard = guardConsole();
  let restoreViewport: () => void;

  beforeEach(() => {
    restoreViewport = installViewport(1280);
  });

  afterEach(() => {
    restoreViewport();
    vi.restoreAllMocks();
  });

  describe('create permission matrix (organization role x policy)', () => {
    const cases: [OrgRole | null, OrgWhoCan, boolean][] = [
      ['owner', 'members', true],
      ['owner', 'admins', true],
      ['admin', 'members', true],
      ['admin', 'admins', true],
      ['member', 'members', true],
      ['member', 'admins', false],
      ['guest', 'members', false],
      ['guest', 'admins', false],
      [null, 'members', false],
    ];

    it.each(cases)(
      'role %s with policy %s shows the create button: %s',
      (role, policy, allowed) => {
        const { fixture, el, store } = setup(role, policy);
        store.teams.set([teamOf('t1', 'Alfa')]);
        fixture.detectChanges();

        expect(createButton(el) !== undefined).toBe(allowed);
      },
    );
  });

  it('shows a loading skeleton with the final card geometry while loading', () => {
    const { fixture, el, store } = setup();
    store.status.set('loading');
    fixture.detectChanges();

    const status = el.querySelector('[role="status"][aria-busy="true"]');
    expect(status?.getAttribute('aria-label')).toBe('Cargando equipos');
    expect(el.querySelector('[data-card-grid]')).not.toBeNull();
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows an error with retry that refetches the teams', () => {
    const { fixture, el, store } = setup();
    store.status.set('error');
    store.error.set(apiErrorOf('http.500', 'Fallo del servidor', 500));
    fixture.detectChanges();

    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Fallo del servidor');
    buttonByText(el, 'Reintentar')?.click();
    expect(store.fetchTeams).toHaveBeenCalledTimes(2);
  });

  it('offers the creation CTA in the empty state for allowed roles', () => {
    const { el } = setup();

    expect(el.textContent).toContain('Aún no hay equipos');
    expect(
      Array.from(el.querySelectorAll('button')).filter(
        (b) => b.textContent?.trim() === 'Crear equipo',
      ),
    ).toHaveLength(1);
  });

  it('explains the empty state without a CTA for guests', () => {
    const { el } = setup('guest');

    expect(el.textContent).toContain(
      'Pide a un administrador de la organización que cree un equipo.',
    );
    expect(createButton(el)).toBeUndefined();
  });

  it('renders one card per team linking to the team page', () => {
    const { fixture, el, store } = setup();
    store.teams.set([teamOf('t1', 'Plataforma', 'Infraestructura'), teamOf('t2', 'Soporte')]);
    fixture.detectChanges();

    const list = cards(el);
    expect(list).toHaveLength(2);
    expect(list[0].getAttribute('href')).toBe('/t1');
    expect(list[0].textContent).toContain('Infraestructura');
    expect(list[1].textContent).toContain('Sin descripción');
    expect(el.querySelector('[data-card-grid]')).not.toBeNull();
  });

  it('filters the cards with the search box', async () => {
    const { fixture, el, store } = setup();
    store.teams.set([teamOf('t1', 'Plataforma'), teamOf('t2', 'Soporte')]);
    fixture.detectChanges();

    typeInto(el.querySelector<HTMLInputElement>('input[aria-label="Buscar equipos"]')!, 'soporte');

    await settle(fixture, () =>
      expect(cards(el).map((card) => card.getAttribute('href'))).toEqual(['/t2']),
    );
  });

  it('shows a no-match state when the search finds nothing', async () => {
    const { fixture, el, store } = setup();
    store.teams.set([teamOf('t1', 'Plataforma')]);
    fixture.detectChanges();

    typeInto(el.querySelector<HTMLInputElement>('input[aria-label="Buscar equipos"]')!, 'zzz');

    await settle(fixture, () =>
      expect(el.textContent).toContain('Ningún equipo coincide con la búsqueda'),
    );
  });

  describe('create flow', () => {
    function openCreateModal(state: Setup): HTMLElement {
      state.store.teams.set([teamOf('t1', 'Plataforma')]);
      state.fixture.detectChanges();
      createButton(state.el)?.click();
      state.fixture.detectChanges();
      return dialog();
    }

    function save(): void {
      buttonByText(dialog(), 'Guardar')?.click();
    }

    it('validates the name length before calling the server', async () => {
      const state = setup();
      const modal = openCreateModal(state);
      typeInto(modal.querySelector<HTMLInputElement>('#team-name')!, 'A');
      save();

      await settle(state.fixture, () =>
        expect(dialog().textContent).toContain('El nombre debe tener entre 2 y 120 caracteres.'),
      );
      expect(state.store.createTeam).not.toHaveBeenCalled();
    });

    it('creates the team and closes the modal on success', async () => {
      const state = setup();
      const modal = openCreateModal(state);
      typeInto(modal.querySelector<HTMLInputElement>('#team-name')!, 'Operaciones');
      typeInto(modal.querySelector<HTMLTextAreaElement>('#team-description')!, 'Turnos y guardias');
      save();

      await settle(state.fixture, () =>
        expect(state.store.createTeam).toHaveBeenCalledWith({
          name: 'Operaciones',
          description: 'Turnos y guardias',
          avatarColor: null,
        }),
      );
      await settle(state.fixture, () =>
        expect(state.notify.success).toHaveBeenCalledWith('Equipo creado'),
      );
    });

    it('maps team.name_taken to the name field', async () => {
      const state = setup();
      state.store.createTeam.mockReturnValueOnce(
        throwError(() => apiErrorOf('team.name_taken', 'x', 409)),
      );
      const modal = openCreateModal(state);
      typeInto(modal.querySelector<HTMLInputElement>('#team-name')!, 'Plataforma');
      save();

      await settle(state.fixture, () =>
        expect(dialog().textContent).toContain(
          'Ya existe un equipo con ese nombre en la organización.',
        ),
      );
      expect(state.notify.success).not.toHaveBeenCalled();
    });

    it('shows team.creation_not_allowed as an alert inside the modal', async () => {
      const state = setup();
      state.store.createTeam.mockReturnValueOnce(
        throwError(() => apiErrorOf('team.creation_not_allowed', 'No puedes crear equipos.', 403)),
      );
      const modal = openCreateModal(state);
      typeInto(modal.querySelector<HTMLInputElement>('#team-name')!, 'Ventas');
      save();

      await settle(state.fixture, () =>
        expect(dialog().querySelector('[role="alert"]')?.textContent).toContain(
          'No puedes crear equipos.',
        ),
      );
    });
  });
});
