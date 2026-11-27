import { Signal, computed, effect, inject, untracked } from '@angular/core';
import { OrganizationStore, IssueStore } from '@corelog/core';

/** A member of the active organization. */
export interface MemberEntry {
  readonly id: string;
  readonly name: string;
}

/** Names of the members of the active organization, keyed by user id. */
export interface MemberNames {
  /** Member name, a loading placeholder while the list loads, or the unknown-member label. */
  nameOf(userId: string): string;
  readonly isLoading: Signal<boolean>;
  /** Every member once the list has loaded; empty before. Lets pickers open with people already listed. */
  readonly members: Signal<readonly MemberEntry[]>;
}

/**
 * Resolves user ids to names with the members of the active organization. The issue store loads the
 * list once per organization and reloads it when the active organization changes.
 * Call it in an injection context.
 */
export function injectMemberNames(): MemberNames {
  const issues = inject(IssueStore);
  const organizations = inject(OrganizationStore);

  effect(() => {
    const organizationId = organizations.currentOrgId();
    untracked(() => issues.loadMemberNames(organizationId));
  });

  return {
    nameOf: (userId: string) => issues.memberNameOf(userId),
    isLoading: computed(() => issues.membersLoading()),
    members: computed(() =>
      Array.from(issues.memberNames.value() ?? [], ([id, name]) => ({ id, name })),
    ),
  };
}
