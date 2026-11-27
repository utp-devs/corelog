import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { OrgRole } from '../../models/organization.model';
import type { PeopleMember, PersonProfile } from '../../models/people.model';
import type { ProfileUpdate, User } from '../../models/user.model';
import { mapUser, profileUpdateBody, type UserResponseDto } from './auth-api.service';

interface MemberResponseDto {
  readonly user_id: string;
  readonly name: string;
  readonly display_name?: string;
  readonly email: string;
  readonly job_title?: string;
  readonly department?: string;
  readonly has_avatar?: boolean;
  readonly role: OrgRole;
  readonly joined_at: string;
}

interface PersonResponseDto {
  readonly user_id: string;
  readonly name: string;
  readonly display_name?: string;
  readonly email: string;
  readonly job_title?: string;
  readonly department?: string;
  readonly timezone?: string;
  readonly has_avatar?: boolean;
  readonly org_role: OrgRole;
  readonly joined_at: string;
}

/** Missing fields (older API) fall back to empty strings and false, never to undefined. */
function mapMember(dto: MemberResponseDto): PeopleMember {
  return {
    userId: dto.user_id,
    name: dto.name,
    displayName: dto.display_name ?? '',
    email: dto.email,
    jobTitle: dto.job_title ?? '',
    department: dto.department ?? '',
    hasAvatar: dto.has_avatar ?? false,
    role: dto.role,
    joinedAt: dto.joined_at,
  };
}

function mapPerson(dto: PersonResponseDto): PersonProfile {
  return {
    userId: dto.user_id,
    name: dto.name,
    displayName: dto.display_name ?? '',
    email: dto.email,
    jobTitle: dto.job_title ?? '',
    department: dto.department ?? '',
    timezone: dto.timezone || 'UTC',
    hasAvatar: dto.has_avatar ?? false,
    orgRole: dto.org_role,
    joinedAt: dto.joined_at,
  };
}

/** People directory of an organization, and the profile of the signed-in member. */
@Injectable({ providedIn: 'root' })
export class PeopleApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  listMembers(organizationId: string): Observable<PeopleMember[]> {
    return this.http
      .get<{ data: MemberResponseDto[] }>(
        `${this.baseUrl}/organizations/${encodeURIComponent(organizationId)}/members`,
      )
      .pipe(map((response) => response.data.map(mapMember)));
  }

  /** Profile of one member. Fails with 404 when the user is not a member, 403 for guests. */
  getPerson(organizationId: string, userId: string): Observable<PersonProfile> {
    return this.http
      .get<{ data: PersonResponseDto }>(
        `${this.baseUrl}/organizations/${encodeURIComponent(organizationId)}/people/${encodeURIComponent(userId)}`,
      )
      .pipe(map((response) => mapPerson(response.data)));
  }

  getMe(): Observable<User> {
    return this.http
      .get<{ data: UserResponseDto }>(`${this.baseUrl}/users/me`)
      .pipe(map((response) => mapUser(response.data)));
  }

  /** Partial update: only the fields present in the payload are sent. */
  updateMe(payload: ProfileUpdate): Observable<User> {
    return this.http
      .patch<{ data: UserResponseDto }>(`${this.baseUrl}/users/me`, profileUpdateBody(payload))
      .pipe(map((response) => mapUser(response.data)));
  }
}
