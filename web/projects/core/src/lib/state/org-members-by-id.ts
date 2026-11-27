import { Injectable, Signal, computed, effect, inject, signal } from '@angular/core';
import { firstValueFrom, map } from 'rxjs';

import { OrganizationApiService } from '../services/api/organization-api.service';
import { resetOnLogout } from './session-scoped';

export interface OrgMemberEntry {
  readonly name: string;
}

/** Organization members indexed by user id; only the name is needed to show a member. */
export type OrgMembersById = ReadonlyMap<string, OrgMemberEntry>;
export type OrgMembersByIdStatus = 'idle' | 'loading' | 'ready' | 'error';

const EMPTY: OrgMembersById = new Map();

/**
 * One member request per organization for the whole session. Failed loads are evicted so that the next
 * use can retry. The cache is cleared when the session ends.
 */
@Injectable({ providedIn: 'root' })
export class OrgMembersByIdCache {
  private readonly api = inject(OrganizationApiService);
  private readonly cache = new Map<string, Promise<OrgMembersById>>();

  constructor() {
    resetOnLogout(() => this.cache.clear());
  }

  load(organizationId: string): Promise<OrgMembersById> {
    const cached = this.cache.get(organizationId);
    if (cached) return cached;
    const pending = firstValueFrom(
      this.api
        .listMembers(organizationId)
        .pipe(
          map(
            (members) => new Map(members.map((member) => [member.userId, { name: member.name }])),
          ),
        ),
    );
    this.cache.set(organizationId, pending);
    pending.catch(() => {
      if (this.cache.get(organizationId) === pending) this.cache.delete(organizationId);
    });
    return pending;
  }
}

/**
 * Signal helper: map from user id to `{ name }` for the members of an organization, loaded once per
 * organization. Call it in an injection context. A null, undefined or empty id stays idle and empty.
 */
export function useOrgMembersById(organizationId: Signal<string | null | undefined>): {
  readonly members: Signal<OrgMembersById>;
  readonly status: Signal<OrgMembersByIdStatus>;
} {
  const cache = inject(OrgMembersByIdCache);
  const settled = signal<{
    readonly organizationId: string;
    readonly status: 'ready' | 'error';
    readonly members: OrgMembersById;
  } | null>(null);

  effect((onCleanup) => {
    const id = organizationId();
    if (!id) return;
    let active = true;
    onCleanup(() => {
      active = false;
    });
    cache.load(id).then(
      (members) => {
        if (active) settled.set({ organizationId: id, status: 'ready', members });
      },
      () => {
        if (active) settled.set({ organizationId: id, status: 'error', members: EMPTY });
      },
    );
  });

  const current = computed(() => {
    const id = organizationId();
    const state = settled();
    return id && state?.organizationId === id ? state : null;
  });

  return {
    members: computed(() => current()?.members ?? EMPTY),
    status: computed<OrgMembersByIdStatus>(() => {
      if (!organizationId()) return 'idle';
      const state = current();
      return state ? state.status : 'loading';
    }),
  };
}
