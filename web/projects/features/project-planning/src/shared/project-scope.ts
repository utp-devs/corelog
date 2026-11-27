import { DestroyRef, Signal, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { ApiError, ProjectStore } from '@corelog/core';
import { EMPTY, Observable, distinctUntilChanged, filter, map } from 'rxjs';

/** Project scope of a planning page: the project id and the failure of reading the project. */
export interface ProjectScope {
  readonly projectId: Signal<string | null>;
  /** Failure of the project read (access or network). Null while the project is readable. */
  readonly accessError: Signal<ApiError | null>;
  /** Reads the project again and reloads the page data for the current project. */
  reload(): void;
}

/**
 * Project id from the nearest route that declares an `id` parameter. The planning routes are children
 * of the project workspace, which owns the `:id` segment. Emits only non-empty values that changed.
 */
export function projectIdParam(route: ActivatedRoute): Observable<string> {
  const owner = [...route.pathFromRoot].reverse().find((item) => item.snapshot.paramMap.has('id'));
  if (!owner) {
    return EMPTY;
  }
  return owner.paramMap.pipe(
    map((params) => params.get('id') ?? ''),
    filter((id) => id !== ''),
    distinctUntilChanged(),
  );
}

/**
 * Follows the project of the route. For every project id, the project is read to check access and
 * `onProject` loads the page data. A failed read is kept in `accessError`; the page decides how to show it.
 */
export function trackProjectScope(
  route: ActivatedRoute,
  projects: ProjectStore,
  destroyRef: DestroyRef,
  onProject: (projectId: string) => void,
): ProjectScope {
  const projectId = signal<string | null>(null);

  const open = (id: string): void => {
    projectId.set(id);
    projects.loadOne(id);
    onProject(id);
  };

  projectIdParam(route)
    .pipe(takeUntilDestroyed(destroyRef))
    .subscribe((id) => open(id));

  return {
    projectId: projectId.asReadonly(),
    accessError: projects.projectError,
    reload: () => {
      const current = projectId();
      if (current) {
        open(current);
      }
    },
  };
}
