import { HttpClient, HttpParams, HttpResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { AuditEntry, AuditFilters, AuditPage, AuditScope } from '../../models/audit.model';
import { silentRequest } from '../../utils/silent-request';

interface AuditDto {
  readonly id: number;
  readonly organization_id?: string | null;
  readonly project_id?: string | null;
  readonly actor_id?: string | null;
  readonly actor_name?: string;
  readonly action: string;
  readonly entity_type: string;
  readonly entity_id?: string;
  readonly summary?: string;
  readonly before?: unknown;
  readonly after?: unknown;
  readonly ip?: string;
  readonly user_agent?: string;
  readonly request_id?: string;
  readonly created_at: string;
}

function toEntry(dto: AuditDto): AuditEntry {
  return {
    id: dto.id,
    organizationId: dto.organization_id ?? null,
    projectId: dto.project_id ?? null,
    actorId: dto.actor_id ?? null,
    actorName: dto.actor_name ?? '',
    action: dto.action,
    entityType: dto.entity_type,
    entityId: dto.entity_id ?? '',
    summary: dto.summary ?? '',
    before: dto.before ?? null,
    after: dto.after ?? null,
    ip: dto.ip ?? '',
    userAgent: dto.user_agent ?? '',
    requestId: dto.request_id ?? '',
    createdAt: dto.created_at,
  };
}

/** Read-only audit log: one organization or the whole platform, with cursor paging and CSV export. */
@Injectable({ providedIn: 'root' })
export class AuditApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  private path(scope: AuditScope): string {
    return scope.kind === 'platform'
      ? `${this.baseUrl}/admin/audit`
      : `${this.baseUrl}/organizations/${encodeURIComponent(scope.organizationId)}/audit`;
  }

  private params(filters: AuditFilters): HttpParams {
    let params = new HttpParams();
    if (filters.actor) params = params.set('actor', filters.actor);
    if (filters.entityType) params = params.set('entity_type', filters.entityType);
    if (filters.action) params = params.set('action', filters.action);
    if (filters.from) params = params.set('from', filters.from);
    if (filters.to) params = params.set('to', filters.to);
    return params;
  }

  list(
    scope: AuditScope,
    filters: AuditFilters,
    options: { readonly cursor?: string | null; readonly limit?: number } = {},
  ): Observable<AuditPage> {
    let params = this.params(filters).set('limit', options.limit ?? 50);
    if (options.cursor) params = params.set('cursor', options.cursor);
    return this.http
      .get<{
        data: AuditDto[] | null;
        meta?: { next_cursor?: string | null };
      }>(this.path(scope), { params, context: silentRequest() })
      .pipe(
        map((response) => ({
          entries: (response.data ?? []).map(toEntry),
          nextCursor: response.meta?.next_cursor || null,
        })),
      );
  }

  /** CSV with the same filters. */
  exportCsv(scope: AuditScope, filters: AuditFilters): Observable<Blob> {
    return this.http
      .get(`${this.path(scope)}/export`, {
        params: this.params(filters),
        responseType: 'blob',
        observe: 'response',
        context: silentRequest(),
      })
      .pipe(map((response: HttpResponse<Blob>) => response.body ?? new Blob([])));
  }
}
