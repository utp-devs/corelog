import { DestroyRef, Injector, Signal, inject } from '@angular/core';
import { takeUntilDestroyed, toObservable } from '@angular/core/rxjs-interop';
import { ApiError, CurrentProjectStore } from '@corelog/core';
import { Observable, distinctUntilChanged, filter } from 'rxjs';

/** Project scope of a planning page: the project id and the failure of reading the project. */
export interface ProjectScope {
  readonly projectId: Signal<string | null>;
  /** Failure of the project read (access or network). Null while the project is readable. */
  readonly accessError: Signal<ApiError | null>;
  /** Reads the project again and reloads the page data for the current project. */
  reload(): void;
}

/**
 * UUIDs of the project in view, as the project area resolves them from the `:key` route parameter.
 * Emits only non-empty values that changed. Call it in an injection context.
 */
export function projectIdChanges(
  current: CurrentProjectStore = inject(CurrentProjectStore),
  injector: Injector = inject(Injector),
): Observable<string> {
  return toObservable(current.projectId, { injector }).pipe(
    filter((id): id is string => !!id),
    distinctUntilChanged(),
  );
}

/**
 * Follows the project of the route. For every project id, `onProject` loads the page data. A failed
 * read is kept in `accessError`; the page decides how to show it. Call it in an injection context.
 */
export function trackProjectScope(
  current: CurrentProjectStore,
  destroyRef: DestroyRef,
  onProject: (projectId: string) => void,
): ProjectScope {
  projectIdChanges(current)
    .pipe(takeUntilDestroyed(destroyRef))
    .subscribe((id) => onProject(id));

  return {
    projectId: current.projectId,
    accessError: current.error,
    reload: () => {
      const id = current.projectId();
      // Without a project the read failed: a successful retry emits the id and loads the page data.
      current.reload();
      if (id) onProject(id);
    },
  };
}
