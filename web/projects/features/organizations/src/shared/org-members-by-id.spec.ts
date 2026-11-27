import { Signal, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ApiError, OrganizationApiService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { of, throwError } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';

import { memberOf } from '../testing/org-test-kit';
import { OrgMembersByIdCache, useOrgMembersById } from './org-members-by-id';

function setup(listMembers: ReturnType<typeof vi.fn>): void {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      { provide: OrganizationApiService, useValue: { listMembers } },
    ],
  });
}

function useFor(id: Signal<string | null | undefined>) {
  return TestBed.runInInjectionContext(() => useOrgMembersById(id));
}

describe('useOrgMembersById', () => {
  it('maps user ids to names once the organization members load', async () => {
    const listMembers = vi.fn(() =>
      of([memberOf('u1', 'Ana Pérez', 'member'), memberOf('u2', 'Luis Gómez', 'admin')]),
    );
    setup(listMembers);
    const view = useFor(signal<string | null>('o1'));

    expect(view.status()).toBe('loading');
    expect(view.members().size).toBe(0);

    TestBed.flushEffects();
    await vi.waitFor(() => expect(view.status()).toBe('ready'));
    expect(view.members().get('u1')).toEqual({ name: 'Ana Pérez' });
    expect(view.members().get('u2')).toEqual({ name: 'Luis Gómez' });
    expect(view.members().get('missing')).toBeUndefined();
    expect(listMembers).toHaveBeenCalledWith('o1');
  });

  it('requests each organization once and reuses the cached map on later uses', async () => {
    const listMembers = vi.fn(() => of([memberOf('u1', 'Ana Pérez', 'member')]));
    setup(listMembers);

    const first = useFor(signal<string | null>('o1'));
    TestBed.flushEffects();
    await vi.waitFor(() => expect(first.status()).toBe('ready'));

    const second = useFor(signal<string | null>('o1'));
    TestBed.flushEffects();
    await vi.waitFor(() => expect(second.status()).toBe('ready'));
    expect(second.members().get('u1')).toEqual({ name: 'Ana Pérez' });
    expect(listMembers).toHaveBeenCalledTimes(1);
  });

  it('stays idle and empty while the organization is unknown', () => {
    const listMembers = vi.fn(() => of([]));
    setup(listMembers);

    const view = useFor(signal<string | null | undefined>(undefined));
    TestBed.flushEffects();
    expect(view.status()).toBe('idle');
    expect(view.members().size).toBe(0);
    expect(listMembers).not.toHaveBeenCalled();
  });

  it('reports an error with an empty map and retries the request on the next use', async () => {
    const failure: ApiError = { status: 500, code: 'http.500', message: 'boom', details: [] };
    const listMembers = vi
      .fn()
      .mockReturnValueOnce(throwError(() => failure))
      .mockReturnValueOnce(of([memberOf('u1', 'Ana Pérez', 'member')]));
    setup(listMembers);

    const failed = useFor(signal<string | null>('o1'));
    TestBed.flushEffects();
    await vi.waitFor(() => expect(failed.status()).toBe('error'));
    expect(failed.members().size).toBe(0);

    const retried = useFor(signal<string | null>('o1'));
    TestBed.flushEffects();
    await vi.waitFor(() => expect(retried.status()).toBe('ready'));
    expect(retried.members().get('u1')).toEqual({ name: 'Ana Pérez' });
    expect(listMembers).toHaveBeenCalledTimes(2);
    expect(TestBed.inject(OrgMembersByIdCache)).toBeTruthy();
  });
});
