import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, of, tap } from 'rxjs';

import {
  CreateOrganizationInput,
  InvitationCreated,
  InvitationItemInput,
  InvitationPreview,
  Organization,
  OrganizationMembership,
} from '../models/organization.model';
import { OrganizationApiService } from '../services/api/organization-api.service';
import { StorageService } from '../services/storage.service';
import { setOrgIdProvider } from '../utils/org-context';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Persisted id of the active organization, so the selection survives a reload. */
export const CURRENT_ORG_STORAGE_KEY = 'corelog.currentOrg';

/**
 * Organizations of the caller and the active one. The active id is persisted in localStorage and
 * registered as the provider of withOrg(), so API services add organization_id to their requests.
 */
@Injectable({ providedIn: 'root' })
export class OrganizationStore {
  private readonly api = inject(OrganizationApiService);
  private readonly storage = inject(StorageService);

  private readonly orgsState = new AsyncState<Organization[]>();
  private readonly currentIdState = signal<string | null>(this.readPersistedId());
  private readonly logoRevisionState = signal<Readonly<Record<string, number>>>({});

  /** Organizations of the caller, each with the caller's role. */
  readonly orgs = computed(() => this.orgsState.value() ?? []);
  /** True once the list has been fetched successfully at least once. */
  readonly loaded = this.orgsState.hasValue;
  /** True while a list request is in flight. */
  readonly loading = this.orgsState.loading;
  /** Last list failure. */
  readonly error = this.orgsState.error;
  readonly currentOrgId = this.currentIdState.asReadonly();
  readonly currentOrg = computed<Organization | null>(() => {
    const id = this.currentIdState();
    return this.orgs().find((org) => org.id === id) ?? null;
  });
  /** Role of the caller in the active organization. */
  readonly currentRole = computed(() => this.currentOrg()?.role ?? null);
  /** Per organization counter, bumped on every logo change. Used as the logo URL version. */
  readonly logoRevision = this.logoRevisionState.asReadonly();

  constructor() {
    setOrgIdProvider(() => this.currentIdState());
    resetOnLogout(() => this.reset());
  }

  /** Reloads the organizations. Keeps the active organization when the caller still belongs to it. */
  fetchOrgs(): void {
    this.orgsState.load(this.listAndSelect());
  }

  /**
   * Organizations for route guards: resolves from the loaded list, or fetches it once. Errors are
   * re-thrown as ApiError and stored in `error`.
   */
  ensureOrgs(): Observable<Organization[]> {
    const current = this.orgsState.value();
    return current !== undefined ? of(current) : this.orgsState.track(this.listAndSelect());
  }

  setCurrentOrg(id: string | null): void {
    this.select(id);
  }

  /** Records a logo upload or removal: updates hasLogo and bumps the logo revision of that organization. */
  setOrgLogo(id: string, hasLogo: boolean): void {
    this.orgsState.update((list) =>
      list?.map((org) => (org.id === id ? { ...org, hasLogo } : org)),
    );
    this.logoRevisionState.update((revisions) => ({
      ...revisions,
      [id]: (revisions[id] ?? 0) + 1,
    }));
  }

  /** Creates an organization and makes it the active one. */
  createOrg(input: CreateOrganizationInput): Observable<Organization> {
    return this.api.create(input).pipe(
      tap((org) => {
        this.orgsState.update((list) => [
          ...(list ?? []).filter((item) => item.id !== org.id),
          org,
        ]);
        this.select(org.id);
      }),
    );
  }

  /** Removes the caller's membership. When the active organization is left, the first remaining one is selected. */
  leaveOrg(id: string): Observable<void> {
    return this.api.leave(id).pipe(
      tap(() => {
        const remaining = (this.orgsState.value() ?? []).filter((org) => org.id !== id);
        this.orgsState.set(remaining);
        if (this.currentIdState() === id) {
          this.select(remaining[0]?.id ?? null);
        }
      }),
    );
  }

  createInvitations(
    organizationId: string,
    items: readonly InvitationItemInput[],
  ): Observable<InvitationCreated[]> {
    return this.api.createInvitations(organizationId, items);
  }

  previewInvitation(token: string): Observable<InvitationPreview> {
    return this.api.previewInvitation(token);
  }

  acceptInvitation(token: string): Observable<OrganizationMembership> {
    return this.api.acceptInvitation(token);
  }

  /** Clears the list, the error and the persisted selection. Runs on logout. */
  reset(): void {
    this.orgsState.reset();
    this.logoRevisionState.set({});
    this.select(null);
  }

  private listAndSelect(): Observable<Organization[]> {
    return this.api.list().pipe(
      tap((orgs) => {
        const current = this.currentIdState();
        const keep = orgs.some((org) => org.id === current) ? current : (orgs[0]?.id ?? null);
        this.select(keep);
      }),
    );
  }

  private select(id: string | null): void {
    this.currentIdState.set(id);
    if (id) {
      this.storage.setItem(CURRENT_ORG_STORAGE_KEY, id);
    } else {
      this.storage.removeItem(CURRENT_ORG_STORAGE_KEY);
    }
  }

  private readPersistedId(): string | null {
    return this.storage.getItem(CURRENT_ORG_STORAGE_KEY);
  }
}
