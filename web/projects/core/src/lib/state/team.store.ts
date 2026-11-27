import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, map, of, switchMap, tap, throwError } from 'rxjs';

import {
  ApiError,
  CreateTeamInput,
  OrganizationPerson,
  Team,
  TeamMember,
  TeamMemberView,
  TeamRole,
  UpdateTeamInput,
} from '../models';
import { TeamApiService } from '../services/api/team-api.service';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { storeError } from '../utils/store-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

export type TeamStoreStatus = 'idle' | 'loading' | 'error';

function statusOf(loading: boolean, error: ApiError | null): TeamStoreStatus {
  if (loading) return 'loading';
  return error ? 'error' : 'idle';
}

/** Names come from the organization directory. When it is not readable (guests), members show without a name. */
function joinPeople(
  memberships: TeamMember[],
  people: OrganizationPerson[] | null,
): TeamMemberView[] {
  const byId = new Map((people ?? []).map((person) => [person.userId, person] as const));
  return memberships.map((membership) => ({
    ...membership,
    name: byId.get(membership.userId)?.name ?? 'Miembro de la organización',
    email: byId.get(membership.userId)?.email ?? '',
  }));
}

function sortByName(teams: Team[]): Team[] {
  return [...teams].sort((a, b) => a.name.localeCompare(b.name));
}

/**
 * Teams of the active organization, the team opened on the detail page and its members.
 * Reads expose status and error through signals; mutations return cold observables that
 * update the state on success and fail with the normalized ApiError (code kept) on error.
 */
@Injectable({ providedIn: 'root' })
export class TeamStore {
  private readonly api = inject(TeamApiService);
  private readonly organizationIdOf = inject(ORGANIZATION_ID_PROVIDER);

  private readonly teamsRead = new AsyncState<Team[]>();
  private readonly currentRead = new AsyncState<Team>();
  private readonly membersRead = new AsyncState<TeamMemberView[]>();

  /** Last failure of a team operation (list, detail or mutation). Null after a successful call. */
  readonly error = signal<ApiError | null>(null);

  readonly teams = computed(() => this.teamsRead.value() ?? []);
  readonly status = computed(() => statusOf(this.teamsRead.loading(), this.teamsRead.error()));

  readonly currentTeam = computed(() => this.currentRead.value() ?? null);
  readonly currentTeamStatus = computed(() =>
    statusOf(this.currentRead.loading(), this.currentRead.error()),
  );

  readonly members = computed(() => this.membersRead.value() ?? []);
  readonly membersStatus = computed(() =>
    statusOf(this.membersRead.loading(), this.membersRead.error()),
  );
  readonly membersError = this.membersRead.error;

  constructor() {
    resetOnLogout(() => this.reset());
  }

  fetchTeams(): void {
    this.error.set(null);
    this.teamsRead.load(
      this.api
        .list()
        .pipe(
          catchError((error: unknown) => this.fail(error, 'No se pudieron cargar los equipos')),
        ),
    );
  }

  fetchTeam(id: string): void {
    this.currentRead.load(
      this.api
        .get(id)
        .pipe(catchError((error: unknown) => this.fail(error, 'No se pudo cargar el equipo'))),
    );
  }

  createTeam(input: Omit<CreateTeamInput, 'organizationId'>): Observable<Team> {
    const organizationId = this.organizationIdOf();
    if (!organizationId) {
      const error: ApiError = {
        status: 422,
        code: 'team.organization_required',
        message: 'Selecciona una organización antes de crear un equipo.',
        details: [],
      };
      this.error.set(error);
      return throwError(() => error);
    }
    return this.api.create({ ...input, organizationId }).pipe(
      tap((team) => {
        this.teamsRead.update((items) => sortByName([...(items ?? []), team]));
        this.error.set(null);
      }),
      catchError((error: unknown) => this.fail(error, 'No se pudo crear el equipo')),
    );
  }

  updateTeam(id: string, input: UpdateTeamInput): Observable<Team> {
    return this.api.update(id, input).pipe(
      tap((updated) => {
        // Merge: fields the response does not carry (memberCount, myRole) keep their known values.
        this.teamsRead.update((items) =>
          items?.map((team) => (team.id === id ? { ...team, ...updated } : team)),
        );
        if (this.currentRead.value()?.id === id) {
          this.currentRead.update((current) => (current ? { ...current, ...updated } : current));
        }
        this.error.set(null);
      }),
      catchError((error: unknown) => this.fail(error, 'No se pudo actualizar el equipo')),
    );
  }

  removeTeam(id: string): Observable<void> {
    return this.api.remove(id).pipe(
      tap(() => {
        this.teamsRead.update((items) => items?.filter((team) => team.id !== id));
        if (this.currentRead.value()?.id === id) {
          this.currentRead.set(undefined);
        }
        this.error.set(null);
      }),
      catchError((error: unknown) => this.fail(error, 'No se pudo eliminar el equipo')),
    );
  }

  fetchMembers(teamId: string): void {
    this.membersRead.load(this.membersSource(teamId));
  }

  addMember(teamId: string, userId: string, role?: TeamRole): Observable<void> {
    return this.api.addMember(teamId, userId, role, { silent: true }).pipe(
      catchError((error: unknown) => this.fail(error, 'No se pudo agregar el miembro')),
      // The list is refreshed after a successful add; a failed refresh only sets membersError.
      switchMap(() =>
        this.membersRead.track(this.membersSource(teamId)).pipe(catchError(() => of(undefined))),
      ),
      map(() => undefined),
    );
  }

  updateMemberRole(teamId: string, userId: string, role: TeamRole): Observable<void> {
    return this.api.updateMemberRole(teamId, userId, role, { silent: true }).pipe(
      tap((updated) => {
        this.membersRead.update((items) =>
          items?.map((member) =>
            member.userId === userId ? { ...member, role: updated.role } : member,
          ),
        );
        this.error.set(null);
      }),
      map(() => undefined),
      catchError((error: unknown) => this.fail(error, 'No se pudo cambiar el rol del miembro')),
    );
  }

  removeMember(teamId: string, userId: string): Observable<void> {
    return this.api.removeMember(teamId, userId, { silent: true }).pipe(
      tap(() => {
        this.membersRead.update((items) => items?.filter((member) => member.userId !== userId));
        this.error.set(null);
      }),
      map(() => undefined),
      catchError((error: unknown) => this.fail(error, 'No se pudo eliminar el miembro')),
    );
  }

  reset(): void {
    this.teamsRead.reset();
    this.currentRead.reset();
    this.membersRead.reset();
    this.error.set(null);
  }

  /** Memberships of a team joined with the names of the organization directory. */
  private membersSource(teamId: string): Observable<TeamMemberView[]> {
    return this.api.listMembers(teamId).pipe(
      switchMap((memberships) => {
        const organizationId = this.organizationIdOf();
        if (!organizationId) return of(joinPeople(memberships, null));
        return this.api.listOrganizationPeople(organizationId).pipe(
          catchError(() => of(null)),
          map((people) => joinPeople(memberships, people)),
        );
      }),
    );
  }

  private fail(error: unknown, fallbackMessage: string): Observable<never> {
    const apiError = storeError(error, fallbackMessage);
    this.error.set(apiError);
    return throwError(() => apiError);
  }
}
