import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  EmailOutboxEntry,
  EmailOutboxPage,
  EmailOutboxStatus,
} from '../../models/email-outbox.model';
import { backgroundRequest, silentRequest } from '../../utils/silent-request';

interface EntryDto {
  readonly id: string;
  readonly user_id?: string | null;
  readonly to: string;
  readonly kind: string;
  readonly subject: string;
  readonly status: EmailOutboxStatus;
  readonly attempts: number;
  readonly next_attempt_at?: string | null;
  readonly last_error?: string;
  readonly created_at: string;
  readonly sent_at?: string | null;
}

interface ListDto {
  readonly data: readonly EntryDto[];
  readonly meta?: { readonly page?: number; readonly page_size?: number; readonly total?: number };
}

function toEntry(dto: EntryDto): EmailOutboxEntry {
  return {
    id: dto.id,
    userId: dto.user_id ?? null,
    to: dto.to,
    kind: dto.kind,
    subject: dto.subject,
    status: dto.status,
    attempts: dto.attempts,
    nextAttemptAt: dto.next_attempt_at ?? null,
    lastError: dto.last_error ?? '',
    createdAt: dto.created_at,
    sentAt: dto.sent_at ?? null,
  };
}

/** Outgoing email queue (platform administrators). */
@Injectable({ providedIn: 'root' })
export class EmailOutboxApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  /** `background` requests (auto refresh) skip the loading bar. */
  list(
    query: { status?: EmailOutboxStatus; page?: number; pageSize?: number },
    background = false,
  ): Observable<EmailOutboxPage> {
    let params = new HttpParams();
    if (query.status) params = params.set('status', query.status);
    if (query.page) params = params.set('page', query.page);
    if (query.pageSize) params = params.set('page_size', query.pageSize);
    return this.http
      .get<ListDto>(`${this.baseUrl}/admin/email-outbox`, {
        params,
        context: background ? backgroundRequest() : silentRequest(),
      })
      .pipe(
        map((response) => ({
          items: response.data.map(toEntry),
          page: response.meta?.page ?? query.page ?? 1,
          pageSize: response.meta?.page_size ?? query.pageSize ?? response.data.length,
          total: response.meta?.total ?? response.data.length,
        })),
      );
  }

  retry(id: string): Observable<void> {
    return this.http.post<void>(`${this.baseUrl}/admin/email-outbox/${id}/retry`, null, {
      context: silentRequest(),
    });
  }
}
