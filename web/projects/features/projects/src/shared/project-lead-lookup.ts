import { Signal, computed, inject } from '@angular/core';
import { rxResource } from '@angular/core/rxjs-interop';
import {
  OrganizationApiService,
  ProjectLeadMembers,
  ProjectLeadMembersStatus,
} from '@corelog/core';
import { map } from 'rxjs';

const NO_MEMBERS: ProjectLeadMembers = new Map();

export interface ProjectLeadLookup {
  /** Organization members keyed by user id. Empty until the members are ready. */
  readonly members: Signal<ProjectLeadMembers>;
  readonly status: Signal<ProjectLeadMembersStatus>;
}

/**
 * Loads the members of an organization so project leads can be shown by name. A null or empty
 * organization id stays idle. Call it in an injection context.
 */
export function projectLeadLookup(
  organizationId: Signal<string | null | undefined>,
): ProjectLeadLookup {
  const api = inject(OrganizationApiService);

  const resource = rxResource({
    params: () => organizationId() || undefined,
    stream: ({ params }) =>
      api
        .listMembers(params)
        .pipe(map((members) => new Map(members.map((m) => [m.userId, { name: m.name }])))),
  });

  const status = computed<ProjectLeadMembersStatus>(() => {
    if (!organizationId()) return 'idle';
    if (resource.error() !== undefined) return 'error';
    if (resource.isLoading() || resource.value() === undefined) return 'loading';
    return 'ready';
  });

  return {
    status,
    members: computed(() => (status() === 'ready' ? (resource.value() ?? NO_MEMBERS) : NO_MEMBERS)),
  };
}
