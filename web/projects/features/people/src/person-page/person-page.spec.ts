import { HttpErrorResponse } from '@angular/common/http';
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  OrganizationStore,
  PeopleApiService,
  PersonProfile,
  Team,
  TeamApiService,
} from '@corelog/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Observable, of, throwError } from 'rxjs';

import { PersonPage } from './person-page';

const person: PersonProfile = {
  userId: 'u-2',
  name: 'Luis Pardo',
  displayName: 'Luis P.',
  email: 'luis@corelog.test',
  jobTitle: 'Analista',
  department: 'Operaciones',
  timezone: 'America/Bogota',
  hasAvatar: false,
  orgRole: 'member',
  joinedAt: '2026-02-01T00:00:00Z',
};

const team: Team = {
  id: 't-1',
  organizationId: 'org-1',
  name: 'Plataforma',
  description: '',
  avatarColor: null,
  createdBy: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  memberCount: 4,
  myRole: 'member',
};

interface SetupOptions {
  org?: { id: string; name: string } | null;
  getPerson?: (orgId: string, userId: string) => Observable<PersonProfile>;
  listForMember?: (orgId: string, memberId: string) => Observable<Team[]>;
}

function setup(options: SetupOptions = {}) {
  const org = signal(options.org === undefined ? { id: 'org-1', name: 'Acme' } : options.org);
  const getPerson = vi.fn(options.getPerson ?? (() => of(person)));
  const listForMember = vi.fn(options.listForMember ?? (() => of([team])));
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'people/:id', component: PersonPage }]),
      { provide: OrganizationStore, useValue: { currentOrg: org } },
      { provide: PeopleApiService, useValue: { listMembers: vi.fn(), getPerson } },
      { provide: TeamApiService, useValue: { listForMember } },
    ],
  });
  return { getPerson, listForMember };
}

async function render(path = '/people/u-2'): Promise<RouterTestingHarness> {
  return RouterTestingHarness.create(path);
}

/** Polls until the text shows up; the card renders after its async load. */
async function expectText(harness: RouterTestingHarness, text: string): Promise<void> {
  await vi.waitFor(() => {
    harness.detectChanges();
    expect(harness.fixture.nativeElement.textContent).toContain(text);
  });
}

describe('PersonPage', () => {
  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('loads the profile from the people endpoint and the teams of the person', async () => {
    const { getPerson, listForMember } = setup();
    const harness = await render();

    await vi.waitFor(() => {
      harness.detectChanges();
      expect(harness.fixture.nativeElement.querySelector('h1')?.textContent?.trim()).toBe(
        'Luis P.',
      );
    });
    expect(getPerson).toHaveBeenCalledWith('org-1', 'u-2');
    expect(listForMember).toHaveBeenCalledWith('org-1', 'u-2');
    const el = harness.fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Analista');
    expect(el.textContent).toContain('Operaciones');
    expect(el.textContent).toContain('America/Bogota');
    // Wide page with the teams in the side panel (no dead space beside the details).
    expect(el.querySelector('[data-page="person"]')?.getAttribute('data-variant')).toBe('wide');
    expect(el.querySelector('aside[aria-label="Equipos de la persona"]')).not.toBeNull();
  });

  it('shows a header card with the role line, the email and the role lozenge', async () => {
    setup();
    const harness = await render();

    await expectText(harness, 'Luis P.');
    const el = harness.fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.person-page__role-line')?.textContent?.trim()).toBe(
      'Analista · Operaciones',
    );
    expect(el.querySelector('a[href="mailto:luis@corelog.test"]')).not.toBeNull();
    expect(el.querySelector('.person-page__header app-lozenge')?.textContent?.trim()).toBe(
      'Miembro',
    );
  });

  it('leaves out the role line when the person has no job title or department', async () => {
    setup({ getPerson: () => of({ ...person, jobTitle: '', department: '' }) });
    const harness = await render();

    await expectText(harness, 'Luis P.');
    expect(harness.fixture.nativeElement.querySelector('.person-page__role-line')).toBeNull();
  });

  it('renders team chips that link to the team page', async () => {
    setup();
    const harness = await render();

    await expectText(harness, 'Plataforma');
    const link = harness.fixture.nativeElement.querySelector(
      'a[href="/teams/t-1"]',
    ) as HTMLAnchorElement;
    expect(link).not.toBeNull();
    expect(link.textContent).toContain('Plataforma');
    expect(harness.fixture.nativeElement.textContent).toContain('4 miembros');
  });

  it('shows the empty teams message when the person has no team', async () => {
    setup({ listForMember: () => of([]) });
    const harness = await render();

    await expectText(harness, 'Sin equipos.');
  });

  it('keeps the profile when only the team list fails', async () => {
    setup({ listForMember: () => throwError(() => new Error('x')) });
    const harness = await render();

    await expectText(harness, 'No se pudieron cargar los equipos.');
    expect(harness.fixture.nativeElement.textContent).toContain('Luis P.');
  });

  it('shows the forbidden page to guests (403 from the people endpoint)', async () => {
    setup({
      getPerson: () =>
        throwError(() => new HttpErrorResponse({ status: 403, statusText: 'Forbidden' })),
    });
    const harness = await render();

    await expectText(harness, 'No tienes acceso a esta página');
  });

  it('shows the not-found page when the person is not a member (404)', async () => {
    setup({
      getPerson: () =>
        throwError(() => new HttpErrorResponse({ status: 404, statusText: 'Not Found' })),
    });
    const harness = await render();

    await expectText(harness, 'Persona no encontrada');
  });

  it('shows an error with retry and loads again on retry', async () => {
    let calls = 0;
    const { getPerson } = setup({
      getPerson: () => {
        calls += 1;
        return calls === 1
          ? throwError(() => new HttpErrorResponse({ status: 500, statusText: 'Server Error' }))
          : of(person);
      },
    });
    const harness = await render();

    await vi.waitFor(() => {
      harness.detectChanges();
      expect(harness.fixture.nativeElement.querySelector('button')).not.toBeNull();
    });
    const retry = Array.from(
      harness.fixture.nativeElement.querySelectorAll('button') as NodeListOf<HTMLButtonElement>,
    ).find((button) => button.textContent?.includes('Reintentar'));
    retry?.click();

    await expectText(harness, 'Luis P.');
    expect(getPerson).toHaveBeenCalledTimes(2);
  });

  it('shows the no-organization page when no organization is active', async () => {
    setup({ org: null });
    const harness = await render();

    await expectText(harness, 'Persona no encontrada');
  });
});
