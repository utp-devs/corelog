import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, Subscription, catchError, map, of, tap, throwError } from 'rxjs';

import { ApiError } from '../models/api-error.model';
import {
  Invitation,
  InvitationCreated,
  InvitationItemInput,
  InvitationScope,
  Organization,
  OrganizationMember,
  OrganizationPatch,
  OrgRole,
} from '../models/organization.model';
import { OrganizationApiService } from '../services/api/organization-api.service';
import { storeError } from '../utils/store-error';
import { AsyncState } from './async-state';
import { OrganizationStore } from './organization.store';
import { resetOnLogout } from './session-scoped';

export type LoadStatus = 'idle' | 'loading' | 'ready' | 'error';

/**
 * Administration of the active organization: members, invitations, logo, settings and the danger zone.
 * Reads update signals and return void. Writes return an Observable: on success the local lists are
 * updated, on failure `error` is set and the observable re-throws the same ApiError.
 */
@Injectable({ providedIn: 'root' })
export class OrganizationAdminStore {
  private readonly api = inject(OrganizationApiService);
  private readonly orgStore = inject(OrganizationStore);

  private readonly membersState = new AsyncState<OrganizationMember[]>();
  private readonly invitationsState = new AsyncState<Invitation[]>();
  private readonly scopeState = signal<InvitationScope>('pending');
  private readonly createdState = signal<InvitationCreated[]>([]);
  private readonly logoUrlState = signal<string | null>(null);
  private readonly errorState = signal<ApiError | null>(null);
  private logoSubscription: Subscription | null = null;
  private logoObjectUrl: string | null = null;

  readonly members = computed(() => this.membersState.value() ?? []);
  readonly membersStatus = computed<LoadStatus>(() => statusOf(this.membersState));
  readonly membersError = this.membersState.error;

  readonly invitations = computed(() => this.invitationsState.value() ?? []);
  readonly invitationScope = this.scopeState.asReadonly();
  readonly invitationsStatus = computed<LoadStatus>(() => statusOf(this.invitationsState));
  readonly invitationsError = this.invitationsState.error;
  /** Invitations created or resent in this session. Plain tokens exist only here, shown once. */
  readonly createdInvitations = this.createdState.asReadonly();

  /** Object URL of the organization logo, or null when there is none. Revoked when replaced. */
  readonly logoUrl = this.logoUrlState.asReadonly();

  /** Last failure of a write action. The action also re-throws the same ApiError. */
  readonly error = this.errorState.asReadonly();

  constructor() {
    resetOnLogout(() => this.reset());
  }

  fetchMembers(organizationId: string): void {
    this.membersState.load(
      this.api
        .listMembers(organizationId)
        .pipe(
          catchError((error: unknown) =>
            this.readFailure(error, 'No se pudo cargar el directorio.'),
          ),
        ),
    );
  }

  changeMemberRole(organizationId: string, userId: string, role: OrgRole): Observable<void> {
    return this.api.changeMemberRole(organizationId, userId, role).pipe(
      catchError((error: unknown) => this.writeFailure(error, 'No se pudo cambiar el rol.')),
      tap(() => {
        this.membersState.update((list) =>
          list?.map((member) => (member.userId === userId ? { ...member, role } : member)),
        );
      }),
    );
  }

  removeMember(organizationId: string, userId: string): Observable<void> {
    return this.api.removeMember(organizationId, userId).pipe(
      catchError((error: unknown) => this.writeFailure(error, 'No se pudo quitar al miembro.')),
      tap(() => {
        this.membersState.update((list) => list?.filter((member) => member.userId !== userId));
      }),
    );
  }

  transferOwnership(organizationId: string, userId: string): Observable<void> {
    return this.api
      .transferOwnership(organizationId, userId)
      .pipe(
        catchError((error: unknown) =>
          this.writeFailure(error, 'No se pudo transferir la propiedad.'),
        ),
      );
  }

  /** Loads the invitations of one scope and remembers the scope for later refreshes. */
  fetchInvitations(organizationId: string, scope?: InvitationScope): void {
    const next = scope ?? this.scopeState();
    this.scopeState.set(next);
    this.invitationsState.load(
      this.api
        .listInvitations(organizationId, next)
        .pipe(
          catchError((error: unknown) =>
            this.readFailure(error, 'No se pudieron cargar las invitaciones.'),
          ),
        ),
    );
  }

  createInvitations(
    organizationId: string,
    items: readonly InvitationItemInput[],
  ): Observable<InvitationCreated[]> {
    return this.api.createInvitations(organizationId, items).pipe(
      catchError((error: unknown) =>
        this.writeFailure(error, 'No se pudieron enviar las invitaciones.'),
      ),
      tap((created) => {
        this.createdState.set(created);
        this.refreshInvitations(organizationId);
      }),
    );
  }

  /** Resending issues a new link; the previous one stops working. */
  resendInvitation(organizationId: string, invitationId: string): Observable<InvitationCreated> {
    return this.api.resendInvitation(organizationId, invitationId).pipe(
      catchError((error: unknown) =>
        this.writeFailure(error, 'No se pudo reenviar la invitación.'),
      ),
      tap((created) => {
        this.createdState.set([created]);
        this.refreshInvitations(organizationId);
      }),
    );
  }

  revokeInvitation(organizationId: string, invitationId: string): Observable<void> {
    return this.api.revokeInvitation(organizationId, invitationId).pipe(
      catchError((error: unknown) => this.writeFailure(error, 'No se pudo revocar la invitación.')),
      tap(() => this.refreshInvitations(organizationId)),
    );
  }

  clearCreatedInvitations(): void {
    this.createdState.set([]);
  }

  updateOrganization(organizationId: string, patch: OrganizationPatch): Observable<Organization> {
    return this.api
      .updateOrganization(organizationId, patch)
      .pipe(
        catchError((error: unknown) =>
          this.writeFailure(error, 'No se pudo guardar la configuración.'),
        ),
      );
  }

  /**
   * Loads the logo as an object URL. `hasLogo: false` skips the request and clears the preview.
   * A missing logo or a failed request both leave no preview.
   */
  loadLogo(organizationId: string, hasLogo?: boolean): void {
    if (hasLogo === false) {
      this.replaceLogo(null);
      return;
    }
    this.logoSubscription?.unsubscribe();
    this.logoSubscription = this.api
      .getLogo(organizationId)
      .pipe(catchError(() => of(null)))
      .subscribe((blob) => this.replaceLogo(blob ? URL.createObjectURL(blob) : null));
  }

  uploadLogo(organizationId: string, file: File): Observable<void> {
    return this.api.uploadLogo(organizationId, file).pipe(
      catchError((error: unknown) => this.writeFailure(error, 'No se pudo subir el logo.')),
      tap(() => {
        this.orgStore.setOrgLogo(organizationId, true);
        this.loadLogo(organizationId);
      }),
      map(() => undefined),
    );
  }

  removeLogo(organizationId: string): Observable<void> {
    return this.api.deleteLogo(organizationId).pipe(
      catchError((error: unknown) => this.writeFailure(error, 'No se pudo quitar el logo.')),
      tap(() => {
        this.replaceLogo(null);
        this.orgStore.setOrgLogo(organizationId, false);
      }),
    );
  }

  /** Owners only. The slug must be typed again (org.confirmation_mismatch otherwise). */
  deleteOrganization(organizationId: string, confirmSlug: string): Observable<void> {
    return this.api
      .deleteOrganization(organizationId, confirmSlug)
      .pipe(
        catchError((error: unknown) =>
          this.writeFailure(error, 'No se pudo eliminar la organización.'),
        ),
      );
  }

  leaveOrganization(organizationId: string): Observable<void> {
    return this.api
      .leave(organizationId)
      .pipe(
        catchError((error: unknown) =>
          this.writeFailure(error, 'No se pudo salir de la organización.'),
        ),
      );
  }

  /** Clears the data, cancels the requests in flight and revokes the logo URL. */
  reset(): void {
    this.membersState.reset();
    this.invitationsState.reset();
    this.scopeState.set('pending');
    this.createdState.set([]);
    this.errorState.set(null);
    this.logoSubscription?.unsubscribe();
    this.logoSubscription = null;
    this.replaceLogo(null);
  }

  private refreshInvitations(organizationId: string): void {
    this.fetchInvitations(organizationId, this.scopeState());
  }

  private replaceLogo(next: string | null): void {
    if (this.logoObjectUrl) {
      URL.revokeObjectURL(this.logoObjectUrl);
    }
    this.logoObjectUrl = next;
    this.logoUrlState.set(next);
  }

  /** Stores a read failure in its own signal (the AsyncState keeps it) with a Spanish fallback. */
  private readFailure(error: unknown, fallback: string): Observable<never> {
    return throwError(() => storeError(error, fallback));
  }

  /** Stores a write failure in `error` and re-throws it for the caller. */
  private writeFailure(error: unknown, fallback: string): Observable<never> {
    const apiError = storeError(error, fallback);
    this.errorState.set(apiError);
    return throwError(() => apiError);
  }
}

function statusOf(state: AsyncState<unknown>): LoadStatus {
  if (state.loading()) return 'loading';
  if (state.error()) return 'error';
  return state.hasValue() ? 'ready' : 'idle';
}
