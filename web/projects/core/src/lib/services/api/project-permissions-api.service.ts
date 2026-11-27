import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  EffectivePermissions,
  EffectiveRole,
  PermissionGrant,
  PermissionHolder,
  PermissionScheme,
  ProjectMemberRole,
  ProjectTeamRole,
} from '../../models';
import { silentRequest } from '../../utils/silent-request';

interface PermissionSchemeDto {
  project_id: string;
  is_default: boolean;
  grants: { permission: string; holders: PermissionHolder[] }[];
}

interface EffectivePermissionsDto {
  role: EffectiveRole;
  permissions: string[];
}

interface ProjectTeamRoleDto {
  project_id: string;
  team_id: string;
  team_name: string;
  role: ProjectMemberRole;
  created_at: string;
}

/**
 * The running API wraps these answers in the usual data envelope; the contract document lists them
 * bare. Both shapes are accepted so the client works with either.
 */
function unwrap<T>(body: T | { data: T }): T {
  return typeof body === 'object' && body !== null && 'data' in body
    ? (body as { data: T }).data
    : (body as T);
}

function toScheme(dto: PermissionSchemeDto): PermissionScheme {
  return {
    projectId: dto.project_id,
    isDefault: dto.is_default,
    grants: dto.grants.map((grant) => ({
      permission: grant.permission,
      holders: [...grant.holders],
    })),
  };
}

function toTeamRole(dto: ProjectTeamRoleDto): ProjectTeamRole {
  return {
    projectId: dto.project_id,
    teamId: dto.team_id,
    teamName: dto.team_name,
    role: dto.role,
    createdAt: dto.created_at,
  };
}

/**
 * Permission scheme, effective permissions and team roles of a project. The project identifier may be
 * the UUID or the key; the permission endpoints answer the scheme itself (no data envelope), the team
 * role endpoints use the usual envelope.
 */
@Injectable({ providedIn: 'root' })
export class ProjectPermissionsApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  private url(projectId: string, path: string): string {
    return `${this.baseUrl}/projects/${encodeURIComponent(projectId)}${path}`;
  }

  /** Full scheme: one grant per catalog permission. */
  getScheme(projectId: string): Observable<PermissionScheme> {
    return this.http
      .get<PermissionSchemeDto | { data: PermissionSchemeDto }>(this.url(projectId, '/permissions'))
      .pipe(map((body) => toScheme(unwrap(body))));
  }

  /** Replaces the whole scheme. Needs edit_project. Rule violations come back as authorization.invalid_scheme. */
  putScheme(projectId: string, grants: readonly PermissionGrant[]): Observable<PermissionScheme> {
    return this.http
      .put<PermissionSchemeDto | { data: PermissionSchemeDto }>(
        this.url(projectId, '/permissions'),
        {
          grants: grants.map((grant) => ({ permission: grant.permission, holders: grant.holders })),
        },
      )
      .pipe(map((body) => toScheme(unwrap(body))));
  }

  /** Drops the rules of the project and goes back to the default scheme. */
  resetScheme(projectId: string): Observable<PermissionScheme> {
    return this.http
      .post<PermissionSchemeDto | { data: PermissionSchemeDto }>(
        this.url(projectId, '/permissions/reset'),
        null,
      )
      .pipe(map((body) => toScheme(unwrap(body))));
  }

  /** Role and permissions of the caller. With `silent`, a failure is left to the caller (no toast). */
  me(
    projectId: string,
    options: { readonly silent?: boolean } = {},
  ): Observable<EffectivePermissions> {
    return this.http
      .get<EffectivePermissionsDto | { data: EffectivePermissionsDto }>(
        this.url(projectId, '/permissions/me'),
        {
          context: options.silent ? silentRequest() : undefined,
        },
      )
      .pipe(
        map((body) => {
          const dto = unwrap(body);
          return { role: dto.role, permissions: [...dto.permissions] };
        }),
      );
  }

  listTeamRoles(projectId: string): Observable<ProjectTeamRole[]> {
    return this.http
      .get<{ data: ProjectTeamRoleDto[] }>(this.url(projectId, '/team-roles'))
      .pipe(map((response) => response.data.map(toTeamRole)));
  }

  /** Gives the team a role in the project, replacing the one it had. */
  putTeamRole(
    projectId: string,
    teamId: string,
    role: ProjectMemberRole,
    options: { readonly silent?: boolean } = {},
  ): Observable<ProjectTeamRole> {
    return this.http
      .put<{ data: ProjectTeamRoleDto }>(
        this.url(projectId, '/team-roles'),
        { team_id: teamId, role },
        { context: options.silent ? silentRequest() : undefined },
      )
      .pipe(map((response) => toTeamRole(response.data)));
  }

  deleteTeamRole(
    projectId: string,
    teamId: string,
    options: { readonly silent?: boolean } = {},
  ): Observable<void> {
    return this.http
      .delete(this.url(projectId, `/team-roles/${encodeURIComponent(teamId)}`), {
        context: options.silent ? silentRequest() : undefined,
      })
      .pipe(map(() => undefined));
  }
}
