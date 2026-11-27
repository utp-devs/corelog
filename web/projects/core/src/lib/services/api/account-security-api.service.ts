import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { publicRequest } from '../../config/http-context';
import type { AccountSession } from '../../models/auth.model';

interface SessionResponseDto {
  readonly id: string;
  readonly user_agent: string;
  readonly created_at: string;
  readonly last_seen_at: string;
  readonly current: boolean;
}

function mapSession(dto: SessionResponseDto): AccountSession {
  return {
    id: dto.id,
    userAgent: dto.user_agent,
    createdAt: dto.created_at,
    lastSeenAt: dto.last_seen_at,
    current: dto.current,
  };
}

/** Login sessions of the signed-in account and the account deactivation. */
@Injectable({ providedIn: 'root' })
export class AccountSecurityApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  listSessions(): Observable<AccountSession[]> {
    return this.http
      .get<{ data: SessionResponseDto[] }>(`${this.baseUrl}/users/me/sessions`)
      .pipe(map((response) => response.data.map(mapSession)));
  }

  revokeSession(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/users/me/sessions/${id}`);
  }

  revokeOtherSessions(): Observable<void> {
    return this.http.post<void>(`${this.baseUrl}/users/me/sessions/revoke-others`, null);
  }

  deactivateAccount(password: string): Observable<void> {
    return this.http.post<void>(`${this.baseUrl}/users/me/deactivate`, { password });
  }

  /**
   * Closes the session of the given token on the server. The token is sent explicitly because the
   * caller may be clearing the local session at the same time. Failures are the caller's to ignore.
   */
  logout(token: string): Observable<void> {
    return this.http.post<void>(`${this.baseUrl}/auth/logout`, null, {
      headers: { Authorization: `Bearer ${token}` },
      context: publicRequest(),
    });
  }
}
