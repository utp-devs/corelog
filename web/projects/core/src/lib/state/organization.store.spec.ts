import { TestBed } from '@angular/core/testing';
import { Observable, of, throwError } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { ApiError } from '../models/api-error.model';
import { OrgRole, OrgWhoCan, Organization } from '../models/organization.model';
import { OrganizationApiService } from '../services/api/organization-api.service';
import { SessionService } from '../services/session.service';
import { getOrgId } from '../utils/org-context';
import { CURRENT_ORG_STORAGE_KEY, OrganizationStore } from './organization.store';
import { createFakeOrganizationApi, orgWith } from './organization-fixtures';

const listFailure: ApiError = { status: 500, code: 'http.500', message: 'boom', details: [] };

function listing(...orgs: Organization[]): Observable<Organization[]> {
  return of(orgs);
}

describe('OrganizationStore', () => {
  let api: ReturnType<typeof createFakeOrganizationApi>;

  function setup(
    overrides: Readonly<Record<string, (...args: never[]) => unknown>> = {},
  ): OrganizationStore {
    api = createFakeOrganizationApi(overrides);
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), { provide: OrganizationApiService, useValue: api }],
    });
    // A signed-in session keeps the logout reset from running during these tests.
    TestBed.inject(SessionService).startSession('token-1');
    return TestBed.inject(OrganizationStore);
  }

  beforeEach(() => {
    localStorage.clear();
  });

  it('selects the first organization when nothing is persisted', () => {
    const store = setup({ list: () => listing(orgWith('a'), orgWith('b')) });
    store.fetchOrgs();
    expect(store.currentOrgId()).toBe('a');
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBe('a');
    expect(store.loaded()).toBe(true);
    expect(store.loading()).toBe(false);
  });

  it('keeps the persisted organization when it is still in the list', () => {
    localStorage.setItem(CURRENT_ORG_STORAGE_KEY, 'b');
    const store = setup({ list: () => listing(orgWith('a'), orgWith('b')) });
    expect(store.currentOrgId()).toBe('b');
    store.fetchOrgs();
    expect(store.currentOrgId()).toBe('b');
  });

  it('falls back to the first organization when the persisted one is gone', () => {
    localStorage.setItem(CURRENT_ORG_STORAGE_KEY, 'removed');
    const store = setup({ list: () => listing(orgWith('a')) });
    store.fetchOrgs();
    expect(store.currentOrgId()).toBe('a');
  });

  it('selects null when the user has no organization', () => {
    localStorage.setItem(CURRENT_ORG_STORAGE_KEY, 'x');
    const store = setup({ list: () => listing() });
    store.fetchOrgs();
    expect(store.currentOrgId()).toBeNull();
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBeNull();
  });

  it('stores a typed error and keeps loaded false when the list fails', () => {
    const store = setup({ list: () => throwError(() => listFailure) });
    store.fetchOrgs();
    expect(store.error()?.code).toBe('http.500');
    expect(store.loaded()).toBe(false);
    expect(store.loading()).toBe(false);
  });

  it('setCurrentOrg persists the choice', () => {
    const store = setup();
    store.setCurrentOrg('z');
    expect(store.currentOrgId()).toBe('z');
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBe('z');
  });

  it('createOrg adds the organization and makes it current', () => {
    const store = setup({
      list: () => listing(orgWith('a')),
      create: () => of(orgWith('new', 'owner')),
    });
    store.fetchOrgs();
    let created: Organization | undefined;
    store.createOrg({ name: 'Nueva' }).subscribe((org) => (created = org));
    expect(created?.id).toBe('new');
    expect(store.orgs().map((org) => org.id)).toEqual(['a', 'new']);
    expect(store.currentOrgId()).toBe('new');
  });

  it('leaveOrg removes the organization and selects another one when it was current', () => {
    const store = setup({ list: () => listing(orgWith('a'), orgWith('b')) });
    store.fetchOrgs();
    store.leaveOrg('a').subscribe();
    expect(api.leave).toHaveBeenCalledWith('a');
    expect(store.orgs().map((org) => org.id)).toEqual(['b']);
    expect(store.currentOrgId()).toBe('b');
  });

  it('reset clears the list, the error and the persisted choice', () => {
    const store = setup({ list: () => listing(orgWith('a')) });
    store.fetchOrgs();
    store.reset();
    expect(store.orgs()).toEqual([]);
    expect(store.loaded()).toBe(false);
    expect(store.currentOrgId()).toBeNull();
    expect(localStorage.getItem(CURRENT_ORG_STORAGE_KEY)).toBeNull();
  });

  it('registers the active id for withOrg through getOrgId', () => {
    const store = setup({ list: () => listing(orgWith('a')) });
    expect(getOrgId()).toBeNull();
    store.fetchOrgs();
    expect(getOrgId()).toBe('a');
  });

  it('derives the current organization and the caller role from the list', () => {
    const store = setup({ list: () => listing(orgWith('a', 'admin')) });
    store.fetchOrgs();
    expect(store.currentOrg()?.id).toBe('a');
    expect(store.currentRole()).toBe('admin');
  });

  function orgWithPolicy(role: OrgRole, policy: OrgWhoCan): Organization {
    const org = orgWith('a', role);
    return { ...org, settings: { ...org.settings, whoCanCreateProjects: policy } };
  }

  it('canCreateProject: members create only when the policy opens creation to them', () => {
    const closed = setup({ list: () => listing(orgWithPolicy('member', 'admins')) });
    closed.fetchOrgs();
    expect(closed.canCreateProject()).toBe(false);
  });

  it('canCreateProject: members create when the policy allows it', () => {
    const open = setup({ list: () => listing(orgWithPolicy('member', 'members')) });
    open.fetchOrgs();
    expect(open.canCreateProject()).toBe(true);
  });

  it('canCreateProject: owners always create and guests never do', () => {
    const owner = setup({ list: () => listing(orgWithPolicy('owner', 'admins')) });
    owner.fetchOrgs();
    expect(owner.canCreateProject()).toBe(true);
  });

  it('canCreateProject: guests never create, even when the policy is open', () => {
    const guest = setup({ list: () => listing(orgWithPolicy('guest', 'members')) });
    guest.fetchOrgs();
    expect(guest.canCreateProject()).toBe(false);
  });

  it('setOrgLogo updates hasLogo and bumps the logo revision of that organization only', () => {
    const store = setup({ list: () => listing(orgWith('a'), orgWith('b')) });
    store.fetchOrgs();
    store.setOrgLogo('a', true);
    expect(store.orgs().find((org) => org.id === 'a')?.hasLogo).toBe(true);
    expect(store.orgs().find((org) => org.id === 'b')?.hasLogo).toBe(false);
    expect(store.logoRevision()).toEqual({ a: 1 });
    store.setOrgLogo('a', false);
    expect(store.logoRevision()).toEqual({ a: 2 });
  });

  it('keeps the list and the selection when leaving fails', () => {
    const store = setup({
      list: () => listing(orgWith('a')),
      leave: () => throwError(() => listFailure),
    });
    store.fetchOrgs();
    let failure: ApiError | undefined;
    store.leaveOrg('a').subscribe({ error: (error: ApiError) => (failure = error) });
    expect(failure?.code).toBe('http.500');
    expect(store.orgs().map((org) => org.id)).toEqual(['a']);
    expect(store.currentOrgId()).toBe('a');
  });

  it('ensureOrgs fetches once and answers from the loaded list afterwards', () => {
    const store = setup({ list: () => listing(orgWith('a')) });
    let first: Organization[] | undefined;
    store.ensureOrgs().subscribe((orgs) => (first = orgs));
    expect(first?.map((org) => org.id)).toEqual(['a']);
    store.ensureOrgs().subscribe();
    expect(api.list).toHaveBeenCalledTimes(1);
  });

  it('ensureOrgs re-throws a failed load as ApiError and stores it', () => {
    const store = setup({ list: () => throwError(() => listFailure) });
    let failure: ApiError | undefined;
    store.ensureOrgs().subscribe({ error: (error: ApiError) => (failure = error) });
    expect(failure?.code).toBe('http.500');
    expect(store.error()?.code).toBe('http.500');
  });
});
