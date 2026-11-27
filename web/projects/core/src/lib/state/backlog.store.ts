import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, forkJoin, map, of, switchMap } from 'rxjs';

import type { BacklogItem } from '../models/backlog.model';
import type { Sprint, SprintIssue } from '../models/sprint.model';
import { BacklogApiService } from '../services/api/backlog-api.service';
import { SprintApiService } from '../services/api/sprint-api.service';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

const MESSAGE_FORBIDDEN = 'No tienes permiso para ordenar el backlog de este proyecto.';
const MESSAGE_INVALID_POSITION = 'La posición de la incidencia no es válida.';
const MESSAGE_FAILED = 'No se pudo reordenar el backlog. Intenta de nuevo.';

/** A planned or active sprint with the issues assigned to it, as the backlog page lists it. */
export interface BacklogSprint {
  readonly sprint: Sprint;
  readonly issues: readonly SprintIssue[];
}

/**
 * Project backlog in rank order, with reordering by position, and the open sprints with their issues so
 * the page can show what is already planned apart from what is not.
 */
@Injectable({ providedIn: 'root' })
export class BacklogStore {
  private readonly api = inject(BacklogApiService);
  private readonly sprintApi = inject(SprintApiService);
  private projectId: string | null = null;
  private readonly moveErrorState = signal<string | null>(null);

  readonly items = new AsyncState<BacklogItem[]>();
  /** Active and planned sprints, the active one first. A failed read leaves the list empty. */
  readonly sprints = new AsyncState<BacklogSprint[]>();
  readonly moveError = this.moveErrorState.asReadonly();

  /** Items in rank order. The first item is the one worked first. */
  readonly ordered = computed<readonly BacklogItem[]>(() => this.items.value() ?? []);
  /** Items that belong to no open sprint, still in rank order. */
  readonly unplanned = computed<readonly BacklogItem[]>(() => {
    const planned = new Set(
      (this.sprints.value() ?? []).flatMap((entry) => entry.issues.map((issue) => issue.id)),
    );
    return this.ordered().filter((item) => !planned.has(item.id));
  });
  readonly isEmpty = computed(() => this.items.hasValue() && this.ordered().length === 0);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  load(projectId: string): void {
    this.projectId = projectId;
    this.moveErrorState.set(null);
    this.items.load(this.api.list(projectId));
    this.sprints.load(this.fetchSprints(projectId));
  }

  retry(): void {
    if (this.projectId) {
      this.load(this.projectId);
    }
  }

  /** The "move up" control is disabled on the first row. */
  canMoveUp(index: number): boolean {
    return index > 0;
  }

  /** The "move down" control is disabled on the last row and on unknown rows. */
  canMoveDown(index: number): boolean {
    return index >= 0 && index < this.unplanned().length - 1;
  }

  /**
   * Places the unplanned item at `index` where the unplanned item at `target` is, so a move never
   * counts the issues that sit in a sprint and are not on screen. Out-of-range requests are ignored.
   * The response is the renumbered backlog, which replaces the list.
   */
  move(index: number, target: number): void {
    const projectId = this.projectId;
    const visible = this.unplanned();
    const item = visible[index];
    const destination = visible[target];
    if (!projectId || !item || !destination) {
      return;
    }
    this.moveErrorState.set(null);
    this.api.move(projectId, item.id, this.ordered().indexOf(destination)).subscribe({
      next: (updated) => {
        if (projectId === this.projectId) {
          this.items.set(updated);
        }
      },
      error: (error: unknown) => this.moveErrorState.set(backlogErrorMessage(asApiError(error))),
    });
  }

  clearMoveError(): void {
    this.moveErrorState.set(null);
  }

  reset(): void {
    this.projectId = null;
    this.moveErrorState.set(null);
    this.items.reset();
    this.sprints.reset();
  }

  private fetchSprints(projectId: string): Observable<BacklogSprint[]> {
    return this.sprintApi.list(projectId).pipe(
      switchMap((sprints) => {
        const open = sprints
          .filter((sprint) => sprint.status !== 'closed')
          .sort((a, b) => Number(b.status === 'active') - Number(a.status === 'active'));
        return open.length === 0
          ? of<BacklogSprint[]>([])
          : forkJoin(
              open.map((sprint) =>
                this.sprintApi
                  .listIssues(projectId, sprint.id)
                  .pipe(map((issues) => ({ sprint, issues }))),
              ),
            );
      }),
      catchError(() => of<BacklogSprint[]>([])),
    );
  }
}

function backlogErrorMessage(error: { readonly status: number }): string {
  if (error.status === 403) {
    return MESSAGE_FORBIDDEN;
  }
  if (error.status === 422) {
    return MESSAGE_INVALID_POSITION;
  }
  return MESSAGE_FAILED;
}
