import { TestBed } from '@angular/core/testing';
import { Observable, Subject, of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { provideCorelogTesting } from '../../testing';
import { ApiError } from '../models/api-error.model';
import { InvitationCreated, OrganizationMember } from '../models/organization.model';
import { OrganizationApiService } from '../services/api/organization-api.service';
import { SessionService } from '../services/session.service';
import { OrganizationAdminStore } from './organization-admin.store';
import { OrganizationStore } from './organization.store';
import {
  ORG,
  createFakeOrganizationApi,
  createdOf,
  invitationOf,
  memberOf,
} from './organization-fixtures';

const conflict: ApiError = {
  status: 409,
  code: 'org.last_owner',
  message: 'Es el último propietario.',
  details: [],
};

function failure(error: ApiError): Observable<never> {
  return throwError(() => error);
}

function capture<T>(source: Observable<T>): { error?: ApiError; value?: T } {
  const result: { error?: ApiError; value?: T } = {};
  source.subscribe({
    next: (value) => (result.value = value),
    error: (error: ApiError) => (result.error = error),
  });
  return result;
}

describe('OrganizationAdminStore', () => {
  let api: ReturnType<typeof createFakeOrganizationApi>;

  function setup(
    overrides: Readonly<Record<string, (...args: never[]) => unknown>> = {},
  ): OrganizationAdminStore {
    api = createFakeOrganizationApi(overrides);
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), { provide: OrganizationApiService, useValue: api }],
    });
    TestBed.inject(SessionService).startSession('token-1');
    return TestBed.inject(OrganizationAdminStore);
  }

  beforeEach(() => {
    localStorage.clear();
    // jsdom has no object URL support.
    URL.createObjectURL = vi.fn(() => 'blob:logo-1');
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('loads members and reports the loading status', () => {
    const pending = new Subject<OrganizationMember[]>();
    const store = setup({ listMembers: () => pending.asObservable() });
    store.fetchMembers(ORG.id);
    expect(store.membersStatus()).toBe('loading');
    pending.next([memberOf('u-1', 'Ana', 'owner')]);
    pending.complete();
    expect(store.membersStatus()).toBe('ready');
    expect(store.members()).toHaveLength(1);
  });

  it('stores a load failure as an ApiError without throwing', () => {
    const store = setup({
      listMembers: () =>
        failure({ status: 403, code: 'org.forbidden', message: 'No', details: [] }),
    });
    store.fetchMembers(ORG.id);
    expect(store.membersStatus()).toBe('error');
    expect(store.membersError()?.code).toBe('org.forbidden');
  });

  it('updates the role locally after a successful change', () => {
    const store = setup({ listMembers: () => of([memberOf('u-2', 'Luis', 'member')]) });
    store.fetchMembers(ORG.id);
    capture(store.changeMemberRole(ORG.id, 'u-2', 'admin'));
    expect(api.changeMemberRole).toHaveBeenCalledWith(ORG.id, 'u-2', 'admin');
    expect(store.members()[0].role).toBe('admin');
  });

  it('rejects with the API error and keeps the list when a role change fails (last owner)', () => {
    const store = setup({
      changeMemberRole: () => failure(conflict),
      listMembers: () => of([memberOf('u-1', 'Ana', 'owner')]),
    });
    store.fetchMembers(ORG.id);
    const result = capture(store.changeMemberRole(ORG.id, 'u-1', 'member'));
    expect(result.error).toBe(conflict);
    expect(store.members()[0].role).toBe('owner');
    expect(store.error()?.code).toBe('org.last_owner');
  });

  it('removes a member from the list after success', () => {
    const store = setup({
      listMembers: () => of([memberOf('u-1', 'Ana', 'owner'), memberOf('u-2', 'Luis', 'guest')]),
    });
    store.fetchMembers(ORG.id);
    capture(store.removeMember(ORG.id, 'u-2'));
    expect(store.members().map((m) => m.userId)).toEqual(['u-1']);
  });

  it('keeps invitations in the current scope and refreshes after create', () => {
    const listInvitations = vi.fn(() => of([invitationOf('i-1', 'ana@acme.test')]));
    const store = setup({
      listInvitations,
      createInvitations: () => of([createdOf('i-1', 'ana@acme.test')]),
    });
    store.fetchInvitations(ORG.id, 'all');
    expect(listInvitations).toHaveBeenLastCalledWith(ORG.id, 'all');

    const result = capture(
      store.createInvitations(ORG.id, [{ email: 'ana@acme.test', role: 'member' }]),
    );
    expect(result.value?.[0].acceptUrl).toMatch(/\/invite\//);
    expect(store.createdInvitations()).toHaveLength(1);
    expect(listInvitations).toHaveBeenLastCalledWith(ORG.id, 'all');
    expect(store.invitations()).toHaveLength(1);
  });

  it('replaces the one-time link on resend and clears it on request', () => {
    const store = setup({ resendInvitation: () => of(createdOf('i-2', 'luis@acme.test')) });
    capture(store.resendInvitation(ORG.id, 'i-2'));
    expect(store.createdInvitations().map((i) => i.id)).toEqual(['i-2']);
    store.clearCreatedInvitations();
    expect(store.createdInvitations()).toEqual([]);
  });

  it('defaults the invitation scope to pending', () => {
    const store = setup();
    expect(store.invitationScope()).toBe('pending');
    store.fetchInvitations(ORG.id);
    expect(api.listInvitations).toHaveBeenLastCalledWith(ORG.id, 'pending');
  });

  it('loads the logo as an object URL and revokes the previous one', () => {
    const blob = new Blob(['png'], { type: 'image/png' });
    const store = setup({ getLogo: vi.fn(() => of<Blob | null>(blob)) });
    store.loadLogo(ORG.id);
    expect(store.logoUrl()).toBe('blob:logo-1');

    store.loadLogo(ORG.id);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:logo-1');
  });

  it('treats a missing logo as no logo', () => {
    const store = setup({ getLogo: vi.fn(() => of<Blob | null>(null)) });
    store.loadLogo(ORG.id);
    expect(store.logoUrl()).toBeNull();
  });

  it('skips the request and clears the preview when the organization has no logo', () => {
    const getLogo = vi.fn(() => of<Blob | null>(new Blob(['x'])));
    const store = setup({ getLogo });
    store.loadLogo(ORG.id, false);
    expect(getLogo).not.toHaveBeenCalled();
    expect(store.logoUrl()).toBeNull();
  });

  it('reports logo uploads and removals to the organization list', () => {
    const store = setup({ getLogo: vi.fn(() => of<Blob | null>(new Blob(['x']))) });
    const setOrgLogo = vi.spyOn(TestBed.inject(OrganizationStore), 'setOrgLogo');
    capture(store.uploadLogo(ORG.id, new File(['x'], 'logo.png', { type: 'image/png' })));
    expect(setOrgLogo).toHaveBeenLastCalledWith(ORG.id, true);
    capture(store.removeLogo(ORG.id));
    expect(setOrgLogo).toHaveBeenLastCalledWith(ORG.id, false);
  });

  it('uploads a logo and then reloads it', () => {
    const uploadLogo = vi.fn(() => of('/organizations/org-1/logo'));
    const getLogo = vi.fn(() => of<Blob | null>(new Blob(['x'], { type: 'image/webp' })));
    const store = setup({ uploadLogo, getLogo });
    const file = new File(['x'], 'logo.webp', { type: 'image/webp' });
    capture(store.uploadLogo(ORG.id, file));
    expect(uploadLogo).toHaveBeenCalledWith(ORG.id, file);
    expect(getLogo).toHaveBeenCalledWith(ORG.id);
  });

  it('sends the typed slug when deleting the organization', () => {
    const store = setup();
    capture(store.deleteOrganization(ORG.id, ORG.slug));
    expect(api.deleteOrganization).toHaveBeenCalledWith(ORG.id, 'acme');
  });

  it('reset clears data and revokes the logo URL', () => {
    const store = setup({
      getLogo: vi.fn(() => of<Blob | null>(new Blob(['x']))),
      listMembers: () => of([memberOf('u-1', 'Ana', 'owner')]),
    });
    store.loadLogo(ORG.id);
    store.fetchMembers(ORG.id);
    store.reset();
    expect(store.members()).toEqual([]);
    expect(store.logoUrl()).toBeNull();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:logo-1');
  });

  it('keeps the API message of a failed update and exposes it through error', () => {
    const store = setup({
      updateOrganization: () =>
        failure({
          status: 422,
          code: 'validation.failed',
          message: 'Nombre no válido.',
          details: [],
        }),
    });
    const result = capture(store.updateOrganization(ORG.id, { name: '' }));
    expect(result.error?.code).toBe('validation.failed');
    expect(store.error()?.message).toBe('Nombre no válido.');
  });

  it('falls back to the Spanish message when a write fails without an API error', () => {
    const store = setup({ revokeInvitation: () => throwError(() => new Error('network down')) });
    const result = capture(store.revokeInvitation(ORG.id, 'i-1'));
    expect(result.error).toMatchObject({
      code: 'unknown',
      message: 'No se pudo revocar la invitación.',
    });
    expect(store.error()?.message).toBe('No se pudo revocar la invitación.');
  });

  it('does not refresh the invitation list when a create fails', () => {
    const store = setup({
      createInvitations: () =>
        failure({ status: 409, code: 'org.invitation_exists', message: 'Ya existe.', details: [] }),
    });
    const result = capture(
      store.createInvitations(ORG.id, [{ email: 'ana@acme.test', role: 'member' }]),
    );
    expect(result.error?.code).toBe('org.invitation_exists');
    expect(api.listInvitations).not.toHaveBeenCalled();
    expect(store.createdInvitations()).toEqual([]);
  });

  it('returns the created link to the caller of a successful create', () => {
    const created: InvitationCreated[] = [createdOf('i-9', 'x@acme.test')];
    const store = setup({ createInvitations: () => of(created) });
    const result = capture(
      store.createInvitations(ORG.id, [{ email: 'x@acme.test', role: 'guest' }]),
    );
    expect(result.value).toEqual(created);
  });
});
