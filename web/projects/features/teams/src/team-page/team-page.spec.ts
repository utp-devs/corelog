import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import {
  AuthStore,
  NotificationService,
  OrganizationStore,
  OrgRole,
  TeamStore,
  UserDirectoryApiService,
} from '@corelog/core';
import { installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  apiErrorOf,
  fakeAuthStore,
  fakeDirectory,
  fakeOrganizationStore,
  fakeTeamStore,
  FakeTeamStore,
  memberOf,
  stubResizeObserver,
  teamOf,
} from '../testing/team-test-kit';
import { TeamPage } from './team-page';

const MEMBERS = [
  memberOf('me', 'Ana Pérez', 'member', 'ana@example.com'),
  memberOf('u2', 'Luis Soto', 'maintainer', 'luis@example.com'),
];

interface Setup {
  fixture: ComponentFixture<TeamPage>;
  el: HTMLElement;
  store: FakeTeamStore;
  router: Router;
  notify: { success: ReturnType<typeof vi.fn>; error: ReturnType<typeof vi.fn> };
}

function setup(
  options: { viewerId?: string; orgRole?: OrgRole | null; path?: string } = {},
): Setup {
  const store = fakeTeamStore();
  store.currentTeam.set({
    ...teamOf('team-1', 'Soporte Nivel 1', 'Atención de primera línea'),
    createdBy: 'me',
  });
  store.members.set(MEMBERS.map((member) => ({ ...member })));
  const notify = { success: vi.fn(), error: vi.fn(), info: vi.fn() };
  TestBed.configureTestingModule({
    imports: [TeamPage],
    providers: [
      provideCorelogTesting(),
      { provide: ActivatedRoute, useValue: { paramMap: of(convertToParamMap({ id: 'team-1' })) } },
      { provide: TeamStore, useValue: store },
      { provide: AuthStore, useValue: fakeAuthStore(options.viewerId ?? 'me') },
      { provide: OrganizationStore, useValue: fakeOrganizationStore(options.orgRole ?? 'member') },
      { provide: UserDirectoryApiService, useValue: fakeDirectory() },
      { provide: NotificationService, useValue: notify },
    ],
  });
  const router = TestBed.inject(Router);
  vi.spyOn(router, 'navigate').mockResolvedValue(true);
  const fixture = TestBed.createComponent(TeamPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, store, router, notify };
}

/** Lets async submits settle, then re-renders. */
async function settle(fixture: ComponentFixture<TeamPage>, check: () => void): Promise<void> {
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

function buttonByLabel(root: ParentNode, label: string): HTMLButtonElement | null {
  return root.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
}

function dialog(): HTMLElement {
  const found = document.querySelector<HTMLElement>('[role="dialog"]');
  if (!found) throw new Error('dialog not open');
  return found;
}

/** Opens the p-select with the given aria label and clicks the option with the given text. */
async function pickRole(
  fixture: ComponentFixture<TeamPage>,
  el: HTMLElement,
  label: string,
  text: string,
): Promise<void> {
  const combobox = el.querySelector<HTMLElement>(`[role="combobox"][aria-label="${label}"]`);
  if (!combobox) throw new Error(`combobox ${label} not rendered`);
  combobox.click();
  fixture.detectChanges();
  await settle(fixture, () => {
    const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(
      (item) => item.textContent?.trim() === text,
    );
    if (!option) throw new Error(`option ${text} not open`);
    option.click();
  });
}

describe('TeamPage', () => {
  let restoreViewport: () => void;

  beforeEach(() => {
    restoreViewport = installViewport(1280);
    stubResizeObserver();
  });

  afterEach(() => {
    restoreViewport();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('loads the team and its members for the route id', () => {
    const { el, store } = setup();

    expect(store.fetchTeam).toHaveBeenCalledWith('team-1');
    expect(store.fetchMembers).toHaveBeenCalledWith('team-1');
    expect(el.querySelector('h1')?.textContent?.trim()).toBe('Soporte Nivel 1');
  });

  it('only offers the members tab; projects and calendar tabs are not shown', () => {
    const { el } = setup();

    const tabs = Array.from(el.querySelectorAll('[role="tab"]')).map((tab) =>
      tab.textContent?.trim(),
    );
    expect(tabs).toEqual(['Miembros']);
  });

  it('shows the details panel with the creator and creation date', () => {
    const { el } = setup();

    const details = el.querySelector('aside[aria-label="Panel lateral"]') as HTMLElement;
    expect(details).not.toBeNull();
    expect(details.textContent).toContain('Detalles');
    expect(details.textContent).toContain('Ana Pérez');
    expect(details.textContent).toContain('Fecha de creación');
  });

  describe('permissions by role', () => {
    it('lets a team maintainer edit, delete and manage members', () => {
      const { el } = setup({ viewerId: 'u2' });

      // Luis is the maintainer of the team in MEMBERS.
      expect(buttonByText(el, 'Editar equipo')).toBeDefined();
      expect(buttonByText(el, 'Eliminar equipo')).toBeDefined();
      expect(el.querySelector('[role="group"][aria-label="Agregar miembro"]')).not.toBeNull();
      expect(el.querySelector('[role="combobox"][aria-label="Rol de Ana Pérez"]')).not.toBeNull();
      expect(buttonByLabel(el, 'Quitar a Ana Pérez del equipo')).not.toBeNull();
    });

    it('keeps a plain team member read-only', () => {
      const { el } = setup();

      expect(buttonByText(el, 'Editar equipo')).toBeUndefined();
      expect(buttonByText(el, 'Eliminar equipo')).toBeUndefined();
      expect(el.querySelector('[role="group"][aria-label="Agregar miembro"]')).toBeNull();
      expect(el.querySelector('[role="combobox"][aria-label="Rol de Luis Soto"]')).toBeNull();
      expect(el.querySelector('button[aria-label^="Quitar a"]')).toBeNull();
      expect(el.textContent).toContain('Responsable');
      expect(el.textContent).toContain(
        'Solo lectura: los responsables del equipo gestionan a sus miembros.',
      );
    });

    it('lets an organization admin manage a team they do not belong to', () => {
      const { el } = setup({ viewerId: 'someone-else', orgRole: 'admin' });

      expect(buttonByText(el, 'Editar equipo')).toBeDefined();
      expect(el.querySelector('[role="group"][aria-label="Agregar miembro"]')).not.toBeNull();
    });

    it('keeps guests read-only', () => {
      const { el } = setup({ viewerId: 'guest-user', orgRole: 'guest' });

      expect(buttonByText(el, 'Editar equipo')).toBeUndefined();
      expect(el.querySelector('[role="group"][aria-label="Agregar miembro"]')).toBeNull();
    });
  });

  it('changes a member role through the role select', async () => {
    const { fixture, el, store } = setup({ viewerId: 'u2' });

    await pickRole(fixture, el, 'Rol de Ana Pérez', 'Responsable');

    expect(store.updateMemberRole).toHaveBeenCalledWith('team-1', 'me', 'maintainer');
  });

  it('removes a member from the team', async () => {
    const { fixture, el, store, notify } = setup({ viewerId: 'u2' });

    buttonByLabel(el, 'Quitar a Ana Pérez del equipo')?.click();

    await settle(fixture, () => expect(store.removeMember).toHaveBeenCalledWith('team-1', 'me'));
    await settle(fixture, () => expect(notify.success).toHaveBeenCalled());
  });

  it('explains a forbidden members read instead of showing an empty table', () => {
    const { fixture, el, store } = setup();
    store.membersStatus.set('error');
    store.membersError.set(apiErrorOf('team.forbidden', 'no', 403));
    fixture.detectChanges();

    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'No tienes permiso para ver los miembros de este equipo.',
    );
  });

  it('shows an empty members state', () => {
    const { fixture, el, store } = setup();
    store.members.set([]);
    fixture.detectChanges();

    expect(el.textContent).toContain('Este equipo aún no tiene miembros');
  });

  it('shows a not-found message when the team cannot be read', () => {
    const { fixture, el, store } = setup();
    store.currentTeam.set(null);
    store.currentTeamStatus.set('error');
    store.error.set(apiErrorOf('team.not_found', 'no', 404));
    fixture.detectChanges();

    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Este equipo no existe o no tienes acceso.',
    );
  });

  it('renders member cards instead of a table on mobile', () => {
    restoreViewport();
    restoreViewport = installViewport(390);
    const { el } = setup({ viewerId: 'u2' });

    expect(el.querySelector('table')).toBeNull();
    expect(el.textContent).toContain('Ana Pérez');
    expect(buttonByLabel(el, 'Quitar a Ana Pérez del equipo')).not.toBeNull();
  });

  describe('delete confirmation', () => {
    it('does not delete until the team name is typed, then removes and returns to the list', async () => {
      const { fixture, el, store, router } = setup({ viewerId: 'u2' });

      buttonByText(el, 'Eliminar equipo')?.click();
      fixture.detectChanges();
      const modal = dialog();
      const confirm = buttonByText(modal, 'Eliminar equipo') as HTMLButtonElement;
      expect(confirm.disabled).toBe(true);

      const field = modal.querySelector<HTMLInputElement>('input[type="text"]')!;
      field.value = 'Soporte Nivel 1';
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
      confirm.click();

      await settle(fixture, () => expect(store.removeTeam).toHaveBeenCalledWith('team-1'));
      await settle(fixture, () =>
        expect(router.navigate).toHaveBeenCalledWith(
          ['..'],
          expect.objectContaining({ replaceUrl: true }),
        ),
      );
    });

    it('keeps the team when the deletion is cancelled', async () => {
      const { fixture, el, store } = setup({ viewerId: 'u2' });

      buttonByText(el, 'Eliminar equipo')?.click();
      fixture.detectChanges();
      buttonByText(dialog(), 'Cancelar')?.click();
      fixture.detectChanges();

      expect(store.removeTeam).not.toHaveBeenCalled();
    });
  });
});
