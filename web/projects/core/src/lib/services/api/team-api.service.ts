import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map, throwError } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  CreateTeamInput,
  OrganizationPerson,
  Team,
  TeamMember,
  TeamRole,
  UpdateTeamInput,
} from '../../models';
import { ORGANIZATION_ID_PROVIDER, noCurrentOrganizationError } from '../../utils/org-id-provider';

interface TeamDto {
  id: string;
  organization_id: string;
  name: string;
  description: string;
  avatar_color: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
  member_count?: number;
  my_role?: TeamRole | null;
}

interface TeamMemberDto {
  team_id: string;
  user_id: string;
  role: TeamRole;
  created_at: string;
}

interface OrganizationMemberDto {
  user_id: string;
  name: string;
  email: string;
}

/** Optional fields are only set when the API sends them, so a partial response never overwrites known values. */
function toTeam(dto: TeamDto): Team {
  const team: Team = {
    id: dto.id,
    organizationId: dto.organization_id,
    name: dto.name,
    description: dto.description ?? '',
    avatarColor: dto.avatar_color ?? null,
    createdBy: dto.created_by ?? null,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
  if (dto.member_count !== undefined) team.memberCount = dto.member_count;
  if (dto.my_role !== undefined) team.myRole = dto.my_role;
  return team;
}

function toTeamMember(dto: TeamMemberDto): TeamMember {
  return {
    teamId: dto.team_id,
    userId: dto.user_id,
    role: dto.role,
    createdAt: dto.created_at,
  };
}

@Injectable({ providedIn: 'root' })
export class TeamApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);

  /** Teams of the active organization. Fails with org.no_current when no organization is active. */
  list(): Observable<Team[]> {
    const organizationId = this.organizationId();
    if (!organizationId) return throwError(() => noCurrentOrganizationError());
    const params = new HttpParams().set('organization_id', organizationId);
    return this.http
      .get<{ data: TeamDto[] }>(`${this.baseUrl}/teams`, { params })
      .pipe(map((response) => response.data.map(toTeam)));
  }

  /** Teams of an organization where the given user is a member. */
  listForMember(organizationId: string, memberId: string): Observable<Team[]> {
    const params = new HttpParams()
      .set('organization_id', organizationId)
      .set('member_id', memberId);
    return this.http
      .get<{ data: TeamDto[] }>(`${this.baseUrl}/teams`, { params })
      .pipe(map((response) => response.data.map(toTeam)));
  }

  get(id: string): Observable<Team> {
    return this.http
      .get<{ data: TeamDto }>(`${this.baseUrl}/teams/${id}`)
      .pipe(map((response) => toTeam(response.data)));
  }

  create(input: CreateTeamInput): Observable<Team> {
    const body: Record<string, unknown> = {
      organization_id: input.organizationId,
      name: input.name,
    };
    if (input.description !== undefined) body['description'] = input.description;
    if (input.avatarColor !== undefined) body['avatar_color'] = input.avatarColor;
    return this.http
      .post<{ data: TeamDto }>(`${this.baseUrl}/teams`, body)
      .pipe(map((response) => toTeam(response.data)));
  }

  update(id: string, input: UpdateTeamInput): Observable<Team> {
    const body: Record<string, unknown> = {};
    if (input.name !== undefined) body['name'] = input.name;
    if (input.description !== undefined) body['description'] = input.description;
    if (input.avatarColor !== undefined) body['avatar_color'] = input.avatarColor;
    return this.http
      .patch<{ data: TeamDto }>(`${this.baseUrl}/teams/${id}`, body)
      .pipe(map((response) => toTeam(response.data)));
  }

  remove(id: string): Observable<void> {
    return this.http.delete(`${this.baseUrl}/teams/${id}`).pipe(map(() => undefined));
  }

  listMembers(teamId: string): Observable<TeamMember[]> {
    return this.http
      .get<{ data: TeamMemberDto[] }>(`${this.baseUrl}/teams/${teamId}/members`)
      .pipe(map((response) => response.data.map(toTeamMember)));
  }

  /** The role is omitted from the body when not given, so the server default applies. */
  addMember(teamId: string, userId: string, role?: TeamRole): Observable<TeamMember> {
    const body: Record<string, unknown> = { user_id: userId };
    if (role !== undefined) body['role'] = role;
    return this.http
      .post<{ data: TeamMemberDto }>(`${this.baseUrl}/teams/${teamId}/members`, body)
      .pipe(map((response) => toTeamMember(response.data)));
  }

  updateMemberRole(teamId: string, userId: string, role: TeamRole): Observable<TeamMember> {
    return this.http
      .patch<{ data: TeamMemberDto }>(`${this.baseUrl}/teams/${teamId}/members/${userId}`, { role })
      .pipe(map((response) => toTeamMember(response.data)));
  }

  removeMember(teamId: string, userId: string): Observable<void> {
    return this.http
      .delete(`${this.baseUrl}/teams/${teamId}/members/${userId}`)
      .pipe(map(() => undefined));
  }

  /** People of an organization with their names. Guests may not read it (the server answers 403). */
  listOrganizationPeople(organizationId: string): Observable<OrganizationPerson[]> {
    return this.http
      .get<{ data: OrganizationMemberDto[] }>(
        `${this.baseUrl}/organizations/${organizationId}/members`,
      )
      .pipe(
        map((response) =>
          response.data.map((member) => ({
            userId: member.user_id,
            name: member.name,
            email: member.email,
          })),
        ),
      );
  }
}
