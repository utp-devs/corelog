import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, Subscription, catchError, switchMap, tap, throwError } from 'rxjs';

import type { ApiError } from '../models/api-error.model';
import type { BacklogIssue, BacklogView } from '../models/backlog-v2.model';
import type { Issue } from '../models/issue.model';
import { BacklogV2ApiService } from '../services/api/backlog-v2-api.service';
import { IssueApiService } from '../services/api/issue-api.service';
import { asApiError } from '../utils/api-error';
import {
  BacklogAnchor,
  moveIssuesInBacklog,
  rangeBetween,
  setPointsInBacklog,
} from '../utils/backlog-view-ops';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** How a click changes the selection. */
export type SelectionMode = 'single' | 'toggle' | 'range';

/** Spanish message for a failed backlog change, by the code of the server answer. */
export function backlogErrorMessage(error: ApiError): string {
  switch (error.code) {
    case 'sprint.closed':
      return 'Un sprint cerrado no admite cambios.';
    case 'project.archived':
      return 'El proyecto está archivado y es de solo lectura.';
    default:
      return error.status === 403
        ? 'No tienes permiso para mover estas incidencias.'
        : 'No se pudo guardar el cambio. Se restauró el backlog.';
  }
}

/**
 * Backlog of the project in view: sprint sections, epics, filters, selection and ranking. A move or a
 * points edit shows at once and is undone when the server refuses it.
 */
@Injectable({ providedIn: 'root' })
export class BacklogViewStore {
  private readonly api = inject(BacklogV2ApiService);
  private readonly issues = inject(IssueApiService);

  private projectId: string | null = null;
  private refreshSubscription: Subscription | null = null;

  private readonly epicState = signal<string | null>(null);
  private readonly searchState = signal('');
  private readonly selectedState = signal<readonly string[]>([]);
  private readonly anchorState = signal<string | null>(null);
  private readonly errorState = signal<string | null>(null);
  private readonly announcementState = signal('');

  readonly view = new AsyncState<BacklogView>();
  readonly epicFilter = this.epicState.asReadonly();
  readonly search = this.searchState.asReadonly();
  readonly selectedIds = this.selectedState.asReadonly();
  /** Failure of the last change, in Spanish. */
  readonly changeError = this.errorState.asReadonly();
  readonly announcement = this.announcementState.asReadonly();

  readonly canMove = computed(() => this.view.value()?.canMoveCards ?? false);
  readonly canManageSprints = computed(() => this.view.value()?.canManageSprints ?? false);

  /** The issue shown in the side panel: the only selected one. */
  readonly inspected = computed<BacklogIssue | null>(() => {
    const ids = this.selectedState();
    if (ids.length !== 1) return null;
    return this.findIssue(ids[0]) ?? null;
  });

  constructor() {
    resetOnLogout(() => this.reset());
  }

  open(projectId: string): void {
    this.projectId = projectId;
    this.selectedState.set([]);
    this.anchorState.set(null);
    this.errorState.set(null);
    this.load(true);
  }

  retry(): void {
    this.load(true);
  }

  /** Reads the backlog again behind the screen: nothing is cleared while it loads. */
  refresh(): void {
    this.load(false);
  }

  setEpicFilter(epicId: string | null): void {
    this.epicState.set(epicId === this.epicState() ? null : epicId);
    this.load(false);
  }

  setSearch(text: string): void {
    this.searchState.set(text);
    this.load(false);
  }

  clearError(): void {
    this.errorState.set(null);
  }

  findIssue(issueId: string): BacklogIssue | undefined {
    for (const section of this.view.value()?.sections ?? []) {
      const found = section.issues.find((issue) => issue.id === issueId);
      if (found) return found;
    }
    return undefined;
  }

  /** Selection after a click on a row; shift selects the range inside its section. */
  select(issueId: string, mode: SelectionMode): void {
    if (mode === 'toggle') {
      this.selectedState.update((ids) =>
        ids.includes(issueId) ? ids.filter((id) => id !== issueId) : [...ids, issueId],
      );
      this.anchorState.set(issueId);
      return;
    }
    if (mode === 'range') {
      const anchor = this.anchorState();
      const section = this.view
        .value()
        ?.sections.find((candidate) => candidate.issues.some((issue) => issue.id === issueId));
      if (anchor && section && section.issues.some((issue) => issue.id === anchor)) {
        const range = rangeBetween(
          section.issues.map((issue) => issue.id),
          anchor,
          issueId,
        );
        this.selectedState.set(range);
        return;
      }
    }
    this.selectedState.set([issueId]);
    this.anchorState.set(issueId);
  }

  clearSelection(): void {
    this.selectedState.set([]);
    this.anchorState.set(null);
  }

  /**
   * Moves issues to a section (a sprint id, or null for the backlog) next to an anchor. The backlog
   * changes at once and goes back if the server refuses.
   */
  rank(issueIds: readonly string[], sprintId: string | null, anchor: BacklogAnchor = {}): void {
    const current = this.view.value();
    if (!current || !this.canMove() || issueIds.length === 0) return;
    const snapshot = current;
    this.refreshSubscription?.unsubscribe();
    this.errorState.set(null);
    this.view.set(moveIssuesInBacklog(current, issueIds, sprintId, anchor));
    this.api
      .rank({
        issueIds,
        sprintId,
        beforeIssueId: anchor.beforeIssueId,
        afterIssueId: anchor.afterIssueId,
      })
      .subscribe({
        next: () => {
          this.announcementState.set(
            issueIds.length === 1 ? 'Incidencia movida' : `${issueIds.length} incidencias movidas`,
          );
          this.refresh();
        },
        error: (raw: unknown) => {
          this.view.set(snapshot);
          this.errorState.set(backlogErrorMessage(asApiError(raw)));
        },
      });
  }

  /** Sets the story points of an issue; empty clears them. */
  setPoints(issueId: string, points: number | null): void {
    const current = this.view.value();
    if (!current) return;
    const snapshot = current;
    this.refreshSubscription?.unsubscribe();
    this.errorState.set(null);
    this.view.set(setPointsInBacklog(current, issueId, points));
    this.api.setPoints(issueId, points).subscribe({
      next: () => this.refresh(),
      error: (raw: unknown) => {
        this.view.set(snapshot);
        this.errorState.set(backlogErrorMessage(asApiError(raw)));
      },
    });
  }

  /** Creates an issue at the end of a section: a sprint id, or null for the backlog. */
  createIssue(summary: string, sprintId: string | null): Observable<Issue> {
    const projectId = this.projectId;
    if (!projectId) return throwError(() => new Error('no project'));
    return this.issues.create({ projectId, summary, description: summary }).pipe(
      switchMap((issue) =>
        sprintId
          ? this.api.rank({ issueIds: [issue.id], sprintId }).pipe(switchMap(() => [issue]))
          : [issue],
      ),
      tap(() => this.refresh()),
      catchError((error: unknown) => throwError(() => asApiError(error))),
    );
  }

  reset(): void {
    this.refreshSubscription?.unsubscribe();
    this.view.reset();
    this.projectId = null;
    this.selectedState.set([]);
    this.epicState.set(null);
    this.searchState.set('');
  }

  private load(blocking: boolean): void {
    const projectId = this.projectId;
    if (!projectId) return;
    const source = this.api.get(projectId, {
      epic: this.epicState() ?? undefined,
      q: this.searchState() || undefined,
    });
    if (blocking) {
      this.refreshSubscription?.unsubscribe();
      this.view.load(source);
      return;
    }
    this.refreshSubscription?.unsubscribe();
    this.refreshSubscription = source.subscribe({
      next: (view) => {
        this.view.set(view);
        const known = new Set(view.sections.flatMap((section) => section.issues.map((i) => i.id)));
        this.selectedState.update((ids) => ids.filter((id) => known.has(id)));
      },
      error: () => undefined,
    });
  }
}
