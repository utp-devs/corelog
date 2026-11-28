import { HttpClient, HttpContext, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION, publicRequest } from '../../config/http-context';
import type {
  ConsentRecord,
  ConsentRef,
  ConsentSource,
  ConsentStatus,
  LegalDocument,
  LegalDocumentSummary,
  PublishLegalPayload,
} from '../../models/legal.model';

interface SummaryDto {
  readonly slug: string;
  readonly title: string;
  readonly version: number;
  readonly summary?: string;
  readonly required?: boolean;
  readonly effective_from?: string;
}

interface DocumentDto extends SummaryDto {
  readonly body_markdown?: string;
  readonly published_at?: string | null;
}

interface StatusDto {
  readonly accepted?: readonly { slug: string; version: number; accepted_at: string }[];
  readonly pending?: readonly { slug: string; title: string; version: number }[];
}

interface RecordDto {
  readonly id: string;
  readonly user_id: string;
  readonly slug: string;
  readonly version: number;
  readonly accepted_at: string;
  readonly source: ConsentSource;
  readonly revoked_at?: string | null;
}

function mapSummary(dto: SummaryDto): LegalDocumentSummary {
  return {
    slug: dto.slug,
    title: dto.title,
    version: dto.version,
    summary: dto.summary ?? '',
    required: dto.required ?? false,
    effectiveFrom: dto.effective_from ?? '',
  };
}

export function mapLegalDocument(dto: DocumentDto): LegalDocument {
  return {
    ...mapSummary(dto),
    bodyMarkdown: dto.body_markdown ?? '',
    publishedAt: dto.published_at ?? null,
  };
}

function mapStatus(dto: StatusDto): ConsentStatus {
  return {
    accepted: (dto.accepted ?? []).map((a) => ({
      slug: a.slug,
      version: a.version,
      acceptedAt: a.accepted_at,
    })),
    pending: (dto.pending ?? []).map((p) => ({ ...p })),
  };
}

/** Legal documents (public), the consents of the signed-in account and the platform administration of both. */
@Injectable({ providedIn: 'root' })
export class LegalApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);
  private readonly quiet = (): HttpContext => new HttpContext().set(SKIP_ERROR_NOTIFICATION, true);

  /** Current version of every document, without the body. Public. */
  listDocuments(): Observable<LegalDocumentSummary[]> {
    return this.http
      .get<{ data: SummaryDto[] }>(`${this.baseUrl}/legal/documents`, { context: publicRequest() })
      .pipe(map((r) => r.data.map(mapSummary)));
  }

  /** One document with its body: the current version, or a specific one. Public. */
  getDocument(slug: string, version?: number): Observable<LegalDocument> {
    let params = new HttpParams();
    if (version !== undefined) params = params.set('version', version);
    return this.http
      .get<{ data: DocumentDto }>(`${this.baseUrl}/legal/documents/${encodeURIComponent(slug)}`, {
        params,
        context: publicRequest(),
      })
      .pipe(map((r) => mapLegalDocument(r.data)));
  }

  myConsents(): Observable<ConsentStatus> {
    return this.http
      .get<{ data: StatusDto }>(`${this.baseUrl}/me/consents`, { context: this.quiet() })
      .pipe(map((r) => mapStatus(r.data)));
  }

  recordConsents(
    accepted: readonly ConsentRef[],
    source: ConsentSource,
  ): Observable<ConsentStatus> {
    return this.http
      .post<{ data: StatusDto }>(
        `${this.baseUrl}/me/consents`,
        { accepted, source },
        { context: this.quiet() },
      )
      .pipe(map((r) => mapStatus(r.data)));
  }

  /** Revokes an optional consent (communications). Required documents answer 409. */
  revokeConsent(slug: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/me/consents/${encodeURIComponent(slug)}`, {
      context: this.quiet(),
    });
  }

  adminListDocuments(): Observable<LegalDocument[]> {
    return this.http
      .get<{ data: DocumentDto[] }>(`${this.baseUrl}/admin/legal/documents`, {
        context: this.quiet(),
      })
      .pipe(map((r) => r.data.map(mapLegalDocument)));
  }

  adminPublish(payload: PublishLegalPayload): Observable<LegalDocument> {
    return this.http
      .post<{ data: DocumentDto }>(
        `${this.baseUrl}/admin/legal/documents`,
        {
          slug: payload.slug,
          title: payload.title,
          summary: payload.summary,
          body_markdown: payload.bodyMarkdown,
          required: payload.required,
        },
        { context: this.quiet() },
      )
      .pipe(map((r) => mapLegalDocument(r.data)));
  }

  adminConsents(userId?: string, limit = 100): Observable<ConsentRecord[]> {
    let params = new HttpParams().set('limit', limit);
    if (userId) params = params.set('user', userId);
    return this.http
      .get<{ data: RecordDto[] }>(`${this.baseUrl}/admin/legal/consents`, {
        params,
        context: this.quiet(),
      })
      .pipe(
        map((r) =>
          r.data.map((d) => ({
            id: d.id,
            userId: d.user_id,
            slug: d.slug,
            version: d.version,
            acceptedAt: d.accepted_at,
            source: d.source,
            revokedAt: d.revoked_at ?? null,
          })),
        ),
      );
  }
}
