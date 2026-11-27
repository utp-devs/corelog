import { Injectable, computed, effect, inject, untracked } from '@angular/core';
import { Observable, switchMap, tap, throwError } from 'rxjs';

import {
  OrganizationStatus,
  OrganizationStatusInput,
  OrganizationStatusPatch,
} from '../models/workflow.model';
import { WorkflowApiService } from '../services/api/workflow-api.service';
import { noCurrentOrganizationError } from '../utils/org-id-provider';
import { AsyncState } from './async-state';
import { OrganizationStore } from './organization.store';
import { resetOnLogout } from './session-scoped';

/**
 * Status catalog of the active organization, cached for the workflow editor and the administration
 * screen. Mutations return cold observables that refresh the list and fail with the normalized ApiError.
 */
@Injectable({ providedIn: 'root' })
export class StatusCatalogStore {
  private readonly api = inject(WorkflowApiService);
  private readonly orgs = inject(OrganizationStore);
  private readonly state = new AsyncState<OrganizationStatus[]>();
  private cachedOrgId: string | null = null;

  readonly statuses = computed(() =>
    [...(this.state.value() ?? [])].sort((a, b) => a.position - b.position),
  );
  readonly loaded = this.state.hasValue;
  readonly loading = this.state.loading;
  readonly error = this.state.error;

  constructor() {
    resetOnLogout(() => this.reset());
    effect(() => {
      const id = this.orgs.currentOrgId();
      untracked(() => {
        if (id !== this.cachedOrgId) this.reset();
      });
    });
  }

  ensureLoaded(): void {
    if (this.loaded() || this.loading()) return;
    this.refresh();
  }

  refresh(): void {
    const orgId = this.orgs.currentOrgId();
    if (!orgId) return;
    this.cachedOrgId = orgId;
    this.state.load(this.api.listStatuses(orgId));
  }

  create(input: OrganizationStatusInput): Observable<OrganizationStatus> {
    return this.mutate((orgId) => this.api.createStatus(orgId, input));
  }

  update(id: string, patch: OrganizationStatusPatch): Observable<OrganizationStatus> {
    return this.mutate((orgId) => this.api.updateStatus(orgId, id, patch));
  }

  /** Deletes a status. Without `reassignTo` the server refuses when the status is in use. */
  remove(id: string, reassignTo?: string): Observable<number> {
    return this.mutate((orgId) => this.api.deleteStatus(orgId, id, reassignTo));
  }

  /** Orders the whole catalog. The list changes at once and is restored if the API refuses. */
  reorder(ids: readonly string[]): Observable<OrganizationStatus[]> {
    const orgId = this.orgs.currentOrgId();
    if (!orgId) return throwError(() => noCurrentOrganizationError());
    const previous = this.state.value();
    if (previous) {
      const byId = new Map(previous.map((item) => [item.id, item] as const));
      this.state.set(
        ids
          .map((id, position) => {
            const item = byId.get(id);
            return item ? { ...item, position } : undefined;
          })
          .filter((item): item is OrganizationStatus => item !== undefined),
      );
    }
    return this.api
      .reorderStatuses(orgId, ids)
      .pipe(tap({ next: (items) => this.state.set(items), error: () => this.state.set(previous) }));
  }

  reset(): void {
    this.cachedOrgId = null;
    this.state.reset();
  }

  private mutate<T>(run: (orgId: string) => Observable<T>): Observable<T> {
    const orgId = this.orgs.currentOrgId();
    if (!orgId) return throwError(() => noCurrentOrganizationError());
    return run(orgId).pipe(
      switchMap((result) =>
        this.api.listStatuses(orgId).pipe(
          tap((items) => this.state.set(items)),
          switchMap(() => [result]),
        ),
      ),
    );
  }
}
