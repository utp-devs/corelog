import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { ApiToken, ApiTokenDraft, ApiTokenScope, CreatedApiToken } from '../../models';
import { silentRequest } from '../../utils/silent-request';

interface ApiTokenDto {
  readonly id: string;
  readonly name: string;
  readonly prefix?: string;
  readonly scopes?: ApiTokenScope[] | null;
  readonly expires_at?: string | null;
  readonly last_used_at?: string | null;
  readonly revoked_at?: string | null;
  readonly created_at: string;
  readonly token?: string;
}

function toToken(dto: ApiTokenDto): ApiToken {
  return {
    id: dto.id,
    name: dto.name,
    prefix: dto.prefix ?? '',
    scopes: dto.scopes ?? [],
    expiresAt: dto.expires_at ?? null,
    lastUsedAt: dto.last_used_at ?? null,
    revokedAt: dto.revoked_at ?? null,
    createdAt: dto.created_at,
  };
}

/** Personal access tokens of the signed-in account. The plain token only exists in the creation answer. */
@Injectable({ providedIn: 'root' })
export class ApiTokenApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(): Observable<readonly ApiToken[]> {
    return this.http
      .get<{ data: ApiTokenDto[] | null }>(`${this.baseUrl}/me/api-tokens`, {
        context: silentRequest(),
      })
      .pipe(map((body) => (body.data ?? []).map(toToken)));
  }

  create(draft: ApiTokenDraft): Observable<CreatedApiToken> {
    const body: Record<string, unknown> = { name: draft.name, scopes: draft.scopes };
    if (draft.expiresAt) body['expires_at'] = draft.expiresAt;
    return this.http
      .post<{ data: ApiTokenDto }>(`${this.baseUrl}/me/api-tokens`, body, {
        context: silentRequest(),
      })
      .pipe(map(({ data }) => ({ token: toToken(data), secret: data.token ?? '' })));
  }

  revoke(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/me/api-tokens/${encodeURIComponent(id)}`, {
      context: silentRequest(),
    });
  }
}
