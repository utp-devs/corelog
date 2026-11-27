import { Injectable, computed, inject } from '@angular/core';
import { Observable, catchError, tap, throwError } from 'rxjs';

import { PermissionGrant, PermissionScheme, ProjectMemberRole, ProjectTeamRole } from '../models';
import { ProjectPermissionsApiService } from '../services/api/project-permissions-api.service';
import { storeError } from '../utils/store-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/**
 * Permission scheme and team roles of the project in view. Reads are loaded with `fetch*`; mutations
 * return cold observables that update the state on success and fail with the normalized ApiError
 * (codes such as authorization.invalid_scheme and project.team_not_in_org are kept).
 */
@Injectable({ providedIn: 'root' })
export class ProjectPermissionsStore {
  private readonly api = inject(ProjectPermissionsApiService);

  private readonly schemeRead = new AsyncState<PermissionScheme>();
  private readonly teamRolesRead = new AsyncState<ProjectTeamRole[]>();

  readonly scheme = this.schemeRead.value;
  readonly schemeLoading = this.schemeRead.loading;
  readonly schemeError = this.schemeRead.error;
  readonly isDefault = computed(() => this.schemeRead.value()?.isDefault ?? true);
  readonly grants = computed(() => this.schemeRead.value()?.grants ?? []);

  readonly teamRoles = computed(() => this.teamRolesRead.value() ?? []);
  readonly teamRolesLoading = this.teamRolesRead.loading;
  readonly teamRolesError = this.teamRolesRead.error;

  constructor() {
    resetOnLogout(() => this.reset());
  }

  fetchScheme(projectId: string): void {
    this.schemeRead.load(this.api.getScheme(projectId));
  }

  saveScheme(projectId: string, grants: readonly PermissionGrant[]): Observable<PermissionScheme> {
    return this.api.putScheme(projectId, grants).pipe(
      tap((scheme) => this.schemeRead.set(scheme)),
      catchError((error: unknown) => this.fail(error, 'No se pudieron guardar los permisos')),
    );
  }

  resetScheme(projectId: string): Observable<PermissionScheme> {
    return this.api.resetScheme(projectId).pipe(
      tap((scheme) => this.schemeRead.set(scheme)),
      catchError((error: unknown) => this.fail(error, 'No se pudieron restaurar los permisos')),
    );
  }

  fetchTeamRoles(projectId: string): void {
    this.teamRolesRead.load(this.api.listTeamRoles(projectId));
  }

  setTeamRole(
    projectId: string,
    teamId: string,
    role: ProjectMemberRole,
  ): Observable<ProjectTeamRole> {
    return this.api.putTeamRole(projectId, teamId, role, { silent: true }).pipe(
      tap((teamRole) =>
        this.teamRolesRead.update((items) => [
          ...(items ?? []).filter((item) => item.teamId !== teamRole.teamId),
          teamRole,
        ]),
      ),
      catchError((error: unknown) => this.fail(error, 'No se pudo guardar el rol del equipo')),
    );
  }

  removeTeamRole(projectId: string, teamId: string): Observable<void> {
    return this.api.deleteTeamRole(projectId, teamId, { silent: true }).pipe(
      tap(() =>
        this.teamRolesRead.update((items) => items?.filter((item) => item.teamId !== teamId)),
      ),
      catchError((error: unknown) => this.fail(error, 'No se pudo quitar el rol del equipo')),
    );
  }

  reset(): void {
    this.schemeRead.reset();
    this.teamRolesRead.reset();
  }

  private fail(error: unknown, fallbackMessage: string): Observable<never> {
    return throwError(() => storeError(error, fallbackMessage));
  }
}
