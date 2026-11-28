import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import { ApiError } from '../models/api-error.model';
import { IssuePermission, IssueView, PatchIssueInput } from '../models/issue-view.model';
import { IssueViewApiService } from '../services/api/issue-view-api.service';
import { asApiError } from '../utils/api-error';

/** Server code of an update refused because someone else changed the issue first. */
export const ISSUE_STALE_CODE = 'issue.stale';

/**
 * State of the issue being viewed: the full view (fields, permissions, counters), its label names and
 * the partial updates. It is provided by the page that shows the issue (not in root), so two open
 * views never share state.
 *
 * Updates are optimistic: the view changes at once and goes back to the previous value when the
 * server refuses. When the server answers that the issue is stale, the view is reloaded first so the
 * user sees what changed, and the error is still thrown to the caller.
 */
@Injectable()
export class IssueViewStore {
  private readonly api = inject(IssueViewApiService);

  private readonly viewState = signal<IssueView | null>(null);
  private readonly loadingState = signal(false);
  private readonly errorState = signal<ApiError | null>(null);
  private readonly labelsState = signal<readonly string[]>([]);
  private readonly savingState = signal(0);
  private requestId = 0;

  readonly view = this.viewState.asReadonly();
  readonly loading = this.loadingState.asReadonly();
  readonly error = this.errorState.asReadonly();
  readonly labels = this.labelsState.asReadonly();
  readonly saving = computed(() => this.savingState() > 0);
  readonly permissions = computed<readonly IssuePermission[]>(
    () => this.viewState()?.permissions ?? [],
  );

  can(permission: IssuePermission): boolean {
    return this.permissions().includes(permission);
  }

  /** Loads the issue by key or id; a response that arrives after another load started is ignored. */
  async load(ref: string): Promise<void> {
    const request = ++this.requestId;
    this.loadingState.set(true);
    this.errorState.set(null);
    try {
      const view = await firstValueFrom(this.api.getView(ref));
      if (request !== this.requestId) return;
      this.viewState.set(view);
      void this.loadLabels(view.key, request);
    } catch (error) {
      if (request !== this.requestId) return;
      this.viewState.set(null);
      this.errorState.set(asApiError(error));
    } finally {
      if (request === this.requestId) this.loadingState.set(false);
    }
  }

  /** Reloads the view without clearing it, so the page does not flash. */
  async refresh(): Promise<void> {
    const current = this.viewState();
    if (!current) return;
    const request = ++this.requestId;
    try {
      const view = await firstValueFrom(this.api.getView(current.id));
      if (request === this.requestId) {
        this.viewState.set(view);
        void this.loadLabels(view.key, request);
      }
    } catch {
      // The data on screen stays; the caller shows its own error if it needs one.
    }
  }

  clear(): void {
    this.requestId++;
    this.viewState.set(null);
    this.errorState.set(null);
    this.labelsState.set([]);
    this.loadingState.set(false);
  }

  /**
   * Applies a partial update. `preview` is merged into the view immediately; it is undone when the
   * request fails. Resolves with the updated view and rejects with an ApiError.
   */
  async patch(input: PatchIssueInput, preview: Partial<IssueView> = {}): Promise<IssueView> {
    const before = this.viewState();
    if (!before) throw new Error('No hay una incidencia abierta');
    this.viewState.set({ ...before, ...preview });
    this.savingState.update((count) => count + 1);
    try {
      await firstValueFrom(
        this.api.patch(before.id, { ...input, expectedUpdatedAt: before.updatedAt }),
      );
      // The patch answer carries the issue fields but not the view extras; reload them.
      const view = await firstValueFrom(this.api.getView(before.id));
      this.viewState.set(view);
      if (input.labels !== undefined) this.labelsState.set(input.labels ?? []);
      return view;
    } catch (error) {
      const apiError = asApiError(error);
      this.viewState.set(before);
      if (apiError.code === ISSUE_STALE_CODE) await this.refresh();
      throw apiError;
    } finally {
      this.savingState.update((count) => count - 1);
    }
  }

  setVoted(voted: boolean): void {
    const view = this.viewState();
    if (!view) return;
    this.viewState.set({
      ...view,
      viewerHasVoted: voted,
      counts: { ...view.counts, votes: Math.max(0, view.counts.votes + (voted ? 1 : -1)) },
    });
  }

  adjustCounts(change: Partial<Record<keyof IssueView['counts'], number>>): void {
    const view = this.viewState();
    if (!view) return;
    const counts = { ...view.counts };
    for (const key of Object.keys(change) as (keyof IssueView['counts'])[]) {
      counts[key] = Math.max(0, counts[key] + (change[key] ?? 0));
    }
    this.viewState.set({ ...view, counts });
  }

  private async loadLabels(key: string, request: number): Promise<void> {
    try {
      const labels = await firstValueFrom(this.api.labelsOf(key));
      if (request === this.requestId || this.viewState()?.key === key) this.labelsState.set(labels);
    } catch {
      this.labelsState.set([]);
    }
  }
}
