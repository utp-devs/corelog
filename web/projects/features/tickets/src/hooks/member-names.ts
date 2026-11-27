import { Signal, computed, effect, inject, untracked } from '@angular/core';
import { OrganizationStore, TicketStore } from '@corelog/core';

/** Names of the members of the active organization, keyed by user id. */
export interface MemberNames {
  /** Member name, a loading placeholder while the list loads, or the unknown-member label. */
  nameOf(userId: string): string;
  readonly isLoading: Signal<boolean>;
}

/**
 * Resolves user ids to names with the members of the active organization. The ticket store loads the
 * list once per organization and reloads it when the active organization changes.
 * Call it in an injection context.
 */
export function injectMemberNames(): MemberNames {
  const tickets = inject(TicketStore);
  const organizations = inject(OrganizationStore);

  effect(() => {
    const organizationId = organizations.currentOrgId();
    untracked(() => tickets.loadMemberNames(organizationId));
  });

  return {
    nameOf: (userId: string) => tickets.memberNameOf(userId),
    isLoading: computed(() => tickets.membersLoading()),
  };
}
