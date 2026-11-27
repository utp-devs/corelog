import { Injectable, computed, effect, inject, untracked } from '@angular/core';
import { Observable, switchMap, tap, throwError } from 'rxjs';

import {
  CatalogCreateInput,
  CatalogItem,
  CatalogKind,
  CatalogPatchInput,
  ProjectIssueTypes,
} from '../models/catalog.model';
import { CatalogApiService } from '../services/api/catalog-api.service';
import { noCurrentOrganizationError } from '../utils/org-id-provider';
import { AsyncState } from './async-state';
import { OrganizationStore } from './organization.store';
import { resetOnLogout } from './session-scoped';

/**
 * Issue types, priorities and resolutions of the active organization, cached for the pickers and the
 * administration screen. Reads are signals; mutations return cold observables that refresh the
 * catalog they changed (so usage counts and positions stay true) and fail with the normalized ApiError.
 * The cache is dropped when the organization changes or the session ends.
 */
@Injectable({ providedIn: 'root' })
export class CatalogStore {
  private readonly api = inject(CatalogApiService);
  private readonly orgs = inject(OrganizationStore);

  private readonly states: Readonly<Record<CatalogKind, AsyncState<CatalogItem[]>>> = {
    'issue-types': new AsyncState<CatalogItem[]>(),
    priorities: new AsyncState<CatalogItem[]>(),
    resolutions: new AsyncState<CatalogItem[]>(),
  };

  /** Organization the cached catalogs belong to. */
  private cachedOrgId: string | null = null;

  readonly issueTypes = computed(() => this.states['issue-types'].value() ?? []);
  readonly priorities = computed(() => this.states.priorities.value() ?? []);
  readonly resolutions = computed(() => this.states.resolutions.value() ?? []);

  /** True once the three catalogs have been loaded for the active organization. */
  readonly loaded = computed(() => Object.values(this.states).every((state) => state.hasValue()));
  readonly loading = computed(() => Object.values(this.states).some((state) => state.loading()));
  readonly error = computed(
    () =>
      Object.values(this.states)
        .map((state) => state.error())
        .find((error) => error !== null) ?? null,
  );

  readonly defaultIssueType = computed(
    () => this.issueTypes().find((item) => item.isDefault) ?? this.issueTypes()[0] ?? null,
  );
  readonly defaultPriority = computed(
    () => this.priorities().find((item) => item.isDefault) ?? this.priorities()[0] ?? null,
  );

  constructor() {
    resetOnLogout(() => this.reset());
    effect(() => {
      const id = this.orgs.currentOrgId();
      untracked(() => {
        if (id !== this.cachedOrgId) this.reset();
      });
    });
  }

  items(kind: CatalogKind): CatalogItem[] {
    return this.states[kind].value() ?? [];
  }

  issueTypeById(id: string): CatalogItem | undefined {
    return this.issueTypes().find((item) => item.id === id);
  }

  priorityById(id: string): CatalogItem | undefined {
    return this.priorities().find((item) => item.id === id);
  }

  /** Loads the catalogs of the active organization unless they are already cached or loading. */
  ensureLoaded(): void {
    if (this.loaded() || this.loading()) return;
    this.refreshAll();
  }

  /** Loads the three catalogs again. */
  refreshAll(): void {
    const orgId = this.orgs.currentOrgId();
    if (!orgId) return;
    this.cachedOrgId = orgId;
    for (const kind of Object.keys(this.states) as CatalogKind[]) {
      this.states[kind].load(this.api.list(orgId, kind));
    }
  }

  refresh(kind: CatalogKind): void {
    const orgId = this.orgs.currentOrgId();
    if (!orgId) return;
    this.cachedOrgId = orgId;
    this.states[kind].load(this.api.list(orgId, kind));
  }

  create(kind: CatalogKind, input: CatalogCreateInput): Observable<CatalogItem> {
    return this.mutate(kind, (orgId) => this.api.create(orgId, kind, input));
  }

  update(kind: CatalogKind, id: string, input: CatalogPatchInput): Observable<CatalogItem> {
    return this.mutate(kind, (orgId) => this.api.update(orgId, kind, id, input));
  }

  /** Deletes an item; `reassignTo` receives its issues when it is in use. */
  remove(kind: CatalogKind, id: string, reassignTo?: string): Observable<void> {
    return this.mutate(kind, (orgId) => this.api.remove(orgId, kind, id, reassignTo));
  }

  /** Moves an item to a new index. The list is updated at once and restored when the API refuses. */
  reorder(kind: CatalogKind, ids: readonly string[]): Observable<CatalogItem[]> {
    const orgId = this.orgs.currentOrgId();
    if (!orgId) return throwError(() => noCurrentOrganizationError());
    const state = this.states[kind];
    const previous = state.value();
    if (previous) {
      const byId = new Map(previous.map((item) => [item.id, item] as const));
      state.set(
        ids
          .map((id, position) => {
            const item = byId.get(id);
            return item ? { ...item, position } : undefined;
          })
          .filter((item): item is CatalogItem => item !== undefined),
      );
    }
    return this.api.reorder(orgId, kind, ids).pipe(
      tap({
        next: (items) => state.set(items),
        error: () => state.set(previous),
      }),
    );
  }

  /** Issue types a project uses (all the organization types when it has no own configuration). */
  projectIssueTypes(projectId: string): Observable<ProjectIssueTypes> {
    return this.api.getProjectIssueTypes(projectId);
  }

  saveProjectIssueTypes(
    projectId: string,
    issueTypeIds: readonly string[],
  ): Observable<ProjectIssueTypes> {
    return this.api.replaceProjectIssueTypes(projectId, issueTypeIds);
  }

  reset(): void {
    this.cachedOrgId = null;
    for (const state of Object.values(this.states)) state.reset();
  }

  private mutate<T>(kind: CatalogKind, run: (orgId: string) => Observable<T>): Observable<T> {
    const orgId = this.orgs.currentOrgId();
    if (!orgId) return throwError(() => noCurrentOrganizationError());
    return run(orgId).pipe(
      switchMap((result) =>
        // Refresh the changed catalog; deleting a type also moves issues, so usage counts of the
        // others change too, which the refresh of that catalog covers.
        this.api.list(orgId, kind).pipe(
          tap((items) => this.states[kind].set(items)),
          switchMap(() => [result]),
        ),
      ),
    );
  }
}
