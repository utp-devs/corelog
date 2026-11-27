import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map, switchMap } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { publicRequest, SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import type { AuthSession, LoginPayload, RegisterPayload } from '../../models/auth.model';
import type {
  AppRole,
  ChangePasswordPayload,
  UpdateProfilePayload,
  User,
} from '../../models/user.model';

/** Wire shape of a user (snake_case). Shared by the account, directory and people endpoints. */
export interface UserResponseDto {
  readonly id: string;
  readonly name: string;
  readonly email: string;
  readonly role: AppRole;
  readonly team_id?: string | null;
  readonly display_name?: string;
  readonly job_title?: string;
  readonly department?: string;
  readonly avatar_key?: string | null;
  readonly timezone?: string;
  readonly created_at: string;
  readonly last_login_at?: string | null;
  readonly email_verified_at?: string | null;
}

interface AuthResponseDto {
  readonly token: string;
  readonly user: UserResponseDto;
}

/** Maps the wire user to the client model. Missing optional fields get their defaults. */
export function mapUser(dto: UserResponseDto): User {
  return {
    id: dto.id,
    name: dto.name,
    email: dto.email,
    role: dto.role,
    teamId: dto.team_id ?? null,
    displayName: dto.display_name ?? '',
    jobTitle: dto.job_title ?? '',
    department: dto.department ?? '',
    avatarKey: dto.avatar_key ?? null,
    timezone: dto.timezone || 'UTC',
    createdAt: dto.created_at,
    lastLoginAt: dto.last_login_at ?? null,
    emailVerifiedAt: dto.email_verified_at ?? null,
  };
}

export function mapUsers(dtos: readonly UserResponseDto[]): User[] {
  return dtos.map(mapUser);
}

/**
 * Builds the body of PATCH /users/me. Only the fields present in the payload are sent,
 * so an omitted field keeps its current value on the server.
 */
export function profileUpdateBody(payload: UpdateProfilePayload): Record<string, string> {
  const body: Record<string, string> = {};
  if (payload.name !== undefined) body['name'] = payload.name;
  if (payload.email !== undefined) body['email'] = payload.email;
  if (payload.displayName !== undefined) body['display_name'] = payload.displayName;
  if (payload.jobTitle !== undefined) body['job_title'] = payload.jobTitle;
  if (payload.department !== undefined) body['department'] = payload.department;
  if (payload.timezone !== undefined) body['timezone'] = payload.timezone;
  return body;
}

/** Account endpoints of the signed-in user, and the public login and registration calls. */
@Injectable({ providedIn: 'root' })
export class AuthApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  /** Creates the account, then signs in with the same credentials. */
  register(payload: RegisterPayload): Observable<AuthSession> {
    return this.http
      .post<{ data: UserResponseDto }>(
        `${this.baseUrl}/auth/register`,
        { name: payload.name, email: payload.email, password: payload.password },
        { context: publicRequest() },
      )
      .pipe(switchMap(() => this.login({ email: payload.email, password: payload.password })));
  }

  login(payload: LoginPayload): Observable<AuthSession> {
    return this.http
      .post<{ data: AuthResponseDto }>(
        `${this.baseUrl}/auth/login`,
        { email: payload.email, password: payload.password },
        { context: publicRequest() },
      )
      .pipe(map((response) => ({ token: response.data.token, user: mapUser(response.data.user) })));
  }

  /** Loads the account behind an explicit token (the session may be restored before it is stored). */
  me(token: string): Observable<User> {
    return this.http
      .get<{ data: UserResponseDto }>(`${this.baseUrl}/users/me`, {
        headers: { Authorization: `Bearer ${token}` },
        context: publicRequest(),
      })
      .pipe(map((response) => mapUser(response.data)));
  }

  updateProfile(payload: UpdateProfilePayload): Observable<User> {
    return this.http
      .patch<{ data: UserResponseDto }>(`${this.baseUrl}/users/me`, profileUpdateBody(payload), {
        context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true),
      })
      .pipe(map((response) => mapUser(response.data)));
  }

  changePassword(payload: ChangePasswordPayload): Observable<void> {
    return this.http.post<void>(
      `${this.baseUrl}/users/me/password`,
      { current_password: payload.currentPassword, new_password: payload.newPassword },
      { context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true) },
    );
  }
}
