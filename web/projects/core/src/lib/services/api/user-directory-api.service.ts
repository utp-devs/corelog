import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { AdminSearchQuery, User } from '../../models/user.model';
import { mapUser, mapUsers, type UserResponseDto } from './auth-api.service';

/** Platform user directory: teammate search for the organization and administration of users. */
@Injectable({ providedIn: 'root' })
export class UserDirectoryApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  /** Users of one organization matching the query. The organization is passed explicitly. */
  searchTeammates(organizationId: string, query: string): Observable<User[]> {
    const params = new HttpParams({ fromObject: { q: query, organization_id: organizationId } });
    return this.http
      .get<{ data: UserResponseDto[] }>(`${this.baseUrl}/users/search`, { params })
      .pipe(map((response) => mapUsers(response.data)));
  }

  /** Administrative search. Omitted filters are not sent. */
  search(query: AdminSearchQuery): Observable<User[]> {
    let params = new HttpParams({ fromObject: { q: query.query } });
    if (query.teamId !== undefined) params = params.set('team_id', query.teamId);
    return this.http
      .get<{ data: UserResponseDto[] }>(`${this.baseUrl}/users`, { params })
      .pipe(map((response) => mapUsers(response.data)));
  }

  /** Moves a user to a team, or removes the team with null. */
  setTeam(userId: string, teamId: string | null): Observable<User> {
    return this.http
      .patch<{ data: UserResponseDto }>(`${this.baseUrl}/users/${userId}/team`, { team_id: teamId })
      .pipe(map((response) => mapUser(response.data)));
  }
}
