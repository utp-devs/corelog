import { HttpErrorResponse } from '@angular/common/http';
import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import {
  FORBIDDEN_DIRECTORY_MESSAGE,
  OrganizationStore,
  PeopleApiService,
  PeopleMember,
} from '@corelog/core';
import { guardConsole, installViewport, provideCorelogTesting } from '@corelog/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Observable, of, throwError } from 'rxjs';

import { PeoplePage } from './people-page';

interface Org {
  readonly id: string;
  readonly name: string;
}

const members: PeopleMember[] = [
  {
    userId: 'u-1',
    name: 'Ana Torres',
    displayName: '',
    email: 'ana@corelog.test',
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role: 'owner',
    joinedAt: '2026-01-10T00:00:00Z',
  },
  {
    userId: 'u-2',
    name: 'Luis Pardo',
    displayName: '',
    email: 'luis@corelog.test',
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role: 'member',
    joinedAt: '2026-02-01T00:00:00Z',
  },
  {
    userId: 'u-3',
    name: 'Marta Ruiz',
    displayName: '',
    email: 'marta@corelog.test',
    jobTitle: '',
    department: '',
    hasAvatar: false,
    role: 'guest',
    joinedAt: '2026-03-01T00:00:00Z',
  },
];

interface SetupOptions {
  org?: Org | null;
  listMembers?: (orgId: string) => Observable<PeopleMember[]>;
}

function setup(options: SetupOptions = {}) {
  const org = signal<Org | null>(
    options.org === undefined ? { id: 'org-1', name: 'Acme' } : options.org,
  );
  const listMembers = vi.fn(options.listMembers ?? (() => of(members)));
  TestBed.configureTestingModule({
    providers: [
      ...provideCorelogTesting(),
      { provide: OrganizationStore, useValue: { currentOrg: org } },
      { provide: PeopleApiService, useValue: { listMembers, getPerson: vi.fn() } },
    ],
  });
  return { org, listMembers };
}

function create(): ComponentFixture<PeoplePage> {
  const fixture = TestBed.createComponent(PeoplePage);
  fixture.detectChanges();
  return fixture;
}

/** Polls until the text shows up; the table renders after its async load. */
async function expectText(fixture: ComponentFixture<PeoplePage>, text: string): Promise<void> {
  await vi.waitFor(() => {
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain(text);
  });
}

function headerTexts(fixture: ComponentFixture<PeoplePage>): string[] {
  const cells = fixture.nativeElement.querySelectorAll('th') as NodeListOf<HTMLElement>;
  return Array.from(cells).map((th) => th.textContent?.trim() ?? '');
}

function rowOf(fixture: ComponentFixture<PeoplePage>, name: string): HTMLElement {
  const rows = fixture.nativeElement.querySelectorAll('tr') as NodeListOf<HTMLElement>;
  const row = Array.from(rows).find((tr) => tr.textContent?.includes(name));
  if (!row) throw new Error(`no row for ${name}`);
  return row;
}

function typeInSearch(fixture: ComponentFixture<PeoplePage>, value: string): void {
  const input = fixture.nativeElement.querySelector(
    'input[aria-label="Buscar personas"]',
  ) as HTMLInputElement;
  input.value = value;
  input.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

describe('PeoplePage', () => {
  const consoleGuard = guardConsole();

  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('lists the members with their organization role and email', async () => {
    setup();
    const fixture = create();

    await expectText(fixture, 'Ana Torres');
    expect(fixture.nativeElement.textContent).toContain('Propietario');
    expect(fixture.nativeElement.textContent).toContain('marta@corelog.test');
    // The directory fills the content column: the table takes the remaining height of the page.
    expect(
      fixture.nativeElement.querySelector('[data-page="people"]')?.getAttribute('data-fill'),
    ).toBe('true');
    expect(
      fixture.nativeElement.querySelector('[data-page="people"] [data-fill="true"]'),
    ).not.toBeNull();
    consoleGuard.expectNoConsoleOutput();
  });

  it('shows the job title of each member in the directory', async () => {
    setup({ listMembers: () => of([{ ...members[1], jobTitle: 'Analista de datos' }]) });
    const fixture = create();

    await expectText(fixture, 'Analista de datos');
    expect(headerTexts(fixture)).toContain('Cargo');
  });

  it('shows the avatar, job title and role in the mobile card list', async () => {
    const restoreViewport = installViewport(390);
    try {
      setup({ listMembers: () => of([{ ...members[1], jobTitle: 'Analista de datos' }]) });
      const fixture = create();

      await expectText(fixture, 'Analista de datos');
      expect(headerTexts(fixture)).not.toContain('Cargo');
      expect(fixture.nativeElement.querySelector('[data-initials="LP"]')).not.toBeNull();
      expect(fixture.nativeElement.textContent).toContain('Miembro');
    } finally {
      restoreViewport();
    }
  });

  it('filters the members by name or email from the search box', async () => {
    setup();
    const fixture = create();
    await expectText(fixture, 'Ana Torres');

    typeInSearch(fixture, 'luis@');

    await vi.waitFor(() => {
      fixture.detectChanges();
      expect(fixture.nativeElement.textContent).not.toContain('Ana Torres');
    });
    expect(fixture.nativeElement.textContent).toContain('Luis Pardo');
  });

  it('shows an empty result message when nothing matches', async () => {
    setup();
    const fixture = create();
    await expectText(fixture, 'Ana Torres');

    typeInSearch(fixture, 'zzz');

    await expectText(fixture, 'Sin resultados');
  });

  it('shows the empty state when the organization has no members', async () => {
    setup({ listMembers: () => of([]) });
    const fixture = create();

    await expectText(fixture, 'Aún no hay personas');
  });

  it('shows the forbidden message for guests and retries on request', async () => {
    let calls = 0;
    const { listMembers } = setup({
      listMembers: () => {
        calls += 1;
        return calls === 1
          ? throwError(() => new HttpErrorResponse({ status: 403, statusText: 'Forbidden' }))
          : of(members);
      },
    });
    const fixture = create();

    await expectText(fixture, FORBIDDEN_DIRECTORY_MESSAGE);
    const alert = fixture.nativeElement.querySelector('[role="alert"]') as HTMLElement;
    const retry = Array.from(alert.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Reintentar'),
    );
    retry?.click();
    fixture.detectChanges();

    await expectText(fixture, 'Ana Torres');
    expect(listMembers).toHaveBeenCalledTimes(2);
  });

  it('opens the person card when a row is activated with the keyboard', async () => {
    setup();
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    const fixture = create();
    await expectText(fixture, 'Luis Pardo');

    rowOf(fixture, 'Luis Pardo').dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }),
    );

    expect(navigate).toHaveBeenCalledWith(['u-2'], expect.anything());
  });

  it('asks for an organization when none is active', async () => {
    const { listMembers } = setup({ org: null });
    const fixture = create();

    await expectText(fixture, 'Elige una organización');
    expect(listMembers).not.toHaveBeenCalled();
  });
});
