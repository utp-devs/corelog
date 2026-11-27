import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  AuthStore,
  NotificationService,
  OrganizationStore,
  TeamStore,
  UserDirectoryApiService,
} from '@corelog/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  fakeAuthStore,
  fakeDirectory,
  fakeOrganizationStore,
  fakeTeamStore,
  memberOf,
  stubResizeObserver,
  teamOf,
} from './testing/team-test-kit';
import { TEAMS_ROUTES } from './teams.routes';

describe('TEAMS_ROUTES', () => {
  beforeEach(() => {
    stubResizeObserver();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('declares the list and the detail with Spanish titles and lazy components', () => {
    expect(TEAMS_ROUTES.map((route) => route.path)).toEqual(['', ':id']);
    expect(TEAMS_ROUTES.map((route) => route.title)).toEqual(['Equipos', 'Equipo']);
    TEAMS_ROUTES.forEach((route) => expect(route.loadComponent).toBeTypeOf('function'));
  });

  it('mounts under the host path with team links that stay inside it', async () => {
    const store = fakeTeamStore();
    store.teams.set([teamOf('t1', 'Plataforma')]);
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'workspace/teams', children: TEAMS_ROUTES }]),
        { provide: TeamStore, useValue: store },
        { provide: OrganizationStore, useValue: fakeOrganizationStore('member') },
        { provide: AuthStore, useValue: fakeAuthStore('me') },
        { provide: UserDirectoryApiService, useValue: fakeDirectory() },
        {
          provide: NotificationService,
          useValue: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
        },
      ],
    });

    const harness = await RouterTestingHarness.create('/workspace/teams');
    const link = harness.routeNativeElement?.querySelector('a[data-testid="team-card"]');

    expect(link?.getAttribute('href')).toBe('/workspace/teams/t1');
  });

  it('mounts the detail at the id segment and loads that team', async () => {
    const store = fakeTeamStore();
    store.currentTeam.set(teamOf('team-1', 'Soporte Nivel 1'));
    store.members.set([memberOf('me', 'Ana Pérez', 'member')]);
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'workspace/teams', children: TEAMS_ROUTES }]),
        { provide: TeamStore, useValue: store },
        { provide: OrganizationStore, useValue: fakeOrganizationStore('member') },
        { provide: AuthStore, useValue: fakeAuthStore('me') },
        { provide: UserDirectoryApiService, useValue: fakeDirectory() },
        {
          provide: NotificationService,
          useValue: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
        },
      ],
    });

    const harness = await RouterTestingHarness.create('/workspace/teams/team-1');

    expect(store.fetchTeam).toHaveBeenCalledWith('team-1');
    expect(harness.routeNativeElement?.querySelector('h1')?.textContent?.trim()).toBe(
      'Soporte Nivel 1',
    );
  });
});
