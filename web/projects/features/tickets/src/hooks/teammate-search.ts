import { DestroyRef, Signal, inject, signal } from '@angular/core';
import { OrganizationStore, User, UserDirectoryApiService } from '@corelog/core';
import { Subscription, finalize } from 'rxjs';

/** Delay before a teammate search is sent, in milliseconds. */
export const TEAMMATE_SEARCH_DEBOUNCE_MS = 300;

export interface TeammateSearch {
  readonly results: Signal<readonly User[]>;
  readonly loading: Signal<boolean>;
  /** Debounced search by name or email. An empty query clears the results. */
  search(query: string): void;
}

/** Debounced search of teammates for the assignee picker. Pending searches are cancelled on destroy. */
export function injectTeammateSearch(): TeammateSearch {
  const directory = inject(UserDirectoryApiService);
  const organizations = inject(OrganizationStore);
  const results = signal<readonly User[]>([]);
  const loading = signal(false);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let request: Subscription | undefined;

  const cancelPending = (): void => {
    clearTimeout(timer);
    request?.unsubscribe();
    request = undefined;
  };
  inject(DestroyRef).onDestroy(cancelPending);

  const search = (query: string): void => {
    cancelPending();
    if (!query) {
      results.set([]);
      loading.set(false);
      return;
    }
    loading.set(true);
    timer = setTimeout(() => {
      const organizationId = organizations.currentOrgId();
      if (!organizationId) {
        results.set([]);
        loading.set(false);
        return;
      }
      request = directory
        .searchTeammates(organizationId, query)
        .pipe(finalize(() => loading.set(false)))
        .subscribe({ next: (users) => results.set(users), error: () => undefined });
    }, TEAMMATE_SEARCH_DEBOUNCE_MS);
  };

  return { results: results.asReadonly(), loading: loading.asReadonly(), search };
}
