import { TestBed } from '@angular/core/testing';
import {
  MemberNameEntry,
  MemberNamesSource,
  LOADING_MEMBER_NAME,
  UNKNOWN_MEMBER_NAME,
} from '@corelog/core';
import { Observable, defer, of, throwError } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';

import { setupTickets } from '../testing/ticket-test-kit';
import { injectMemberNames } from './member-names';

/** Lets pending effects and promise callbacks run. */
async function settle(): Promise<void> {
  TestBed.flushEffects();
  await new Promise((done) => setTimeout(done, 0));
  TestBed.flushEffects();
}

function entry(userId: string, name: string): MemberNameEntry {
  return { userId, name };
}

function sourceOf(
  list: (orgId: string) => Observable<MemberNameEntry[]>,
): MemberNamesSource & { listMembers: ReturnType<typeof vi.fn> } {
  return { listMembers: vi.fn(list) } as MemberNamesSource & {
    listMembers: ReturnType<typeof vi.fn>;
  };
}

function nameHelper() {
  return TestBed.runInInjectionContext(() => injectMemberNames());
}

describe('injectMemberNames', () => {
  it('resolves ids to member names and falls back for ids that are not members', async () => {
    const source = sourceOf(() =>
      of([entry('user-1', 'Ana García'), entry('user-2', 'Luis Pérez')]),
    );
    setupTickets({ memberSource: source });
    const names = nameHelper();
    await settle();

    expect(names.nameOf('user-1')).toBe('Ana García');
    expect(names.nameOf('user-2')).toBe('Luis Pérez');
    expect(names.nameOf('removed-user')).toBe(UNKNOWN_MEMBER_NAME);
    expect(source.listMembers).toHaveBeenCalledWith('org-1');
  });

  it('shows a loading placeholder until the member list arrives', async () => {
    let resolve: (list: MemberNameEntry[]) => void = () => undefined;
    const source = sourceOf(() =>
      defer(
        () =>
          new Promise<MemberNameEntry[]>((done) => {
            resolve = done;
          }),
      ),
    );
    setupTickets({ memberSource: source });
    const names = nameHelper();
    await settle();

    expect(names.nameOf('user-1')).toBe(LOADING_MEMBER_NAME);
    expect(names.isLoading()).toBe(true);
    resolve([entry('user-1', 'Ana García')]);
    await settle();
    expect(names.nameOf('user-1')).toBe('Ana García');
    expect(names.isLoading()).toBe(false);
  });

  it('requests the member list once per organization and shares it between helpers', async () => {
    const source = sourceOf(() => of([entry('user-1', 'Ana García')]));
    setupTickets({ memberSource: source });
    const first = nameHelper();
    const second = nameHelper();
    await settle();

    expect(first.nameOf('user-1')).toBe('Ana García');
    expect(second.nameOf('user-1')).toBe('Ana García');
    expect(source.listMembers).toHaveBeenCalledTimes(1);
  });

  it('falls back to the unknown label when the member list cannot be loaded and retries on the next use', async () => {
    const failing = sourceOf(() => throwError(() => new Error('network failure')));
    setupTickets({ memberSource: failing });
    const names = nameHelper();
    await settle();

    expect(names.isLoading()).toBe(false);
    expect(names.nameOf('user-1')).toBe(UNKNOWN_MEMBER_NAME);
  });

  it('does not request anything without an active organization', async () => {
    const source = sourceOf(() => of([entry('user-1', 'Ana García')]));
    const setup = setupTickets({ memberSource: source, orgId: null });
    const names = nameHelper();
    await settle();

    expect(source.listMembers).not.toHaveBeenCalled();
    expect(names.nameOf('user-1')).toBe(UNKNOWN_MEMBER_NAME);
    expect(names.isLoading()).toBe(false);
    expect(setup.organizations.currentOrgId()).toBeNull();
  });

  it('loads the members of the new organization when the active one changes', async () => {
    const source = sourceOf((orgId) =>
      of([entry('user-1', orgId === 'org-1' ? 'Ana García' : 'Ana G. (otra org)')]),
    );
    const setup = setupTickets({ memberSource: source });
    const names = nameHelper();
    await settle();
    expect(names.nameOf('user-1')).toBe('Ana García');

    setup.organizations.currentOrgId.set('org-2');
    await settle();
    expect(names.nameOf('user-1')).toBe('Ana G. (otra org)');
    expect(source.listMembers).toHaveBeenCalledWith('org-2');
  });
});
