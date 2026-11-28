import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  CreatedWebhook,
  Webhook,
  WebhookDelivery,
  WebhookDeliveryStatus,
  WebhookDraft,
} from '../../models';
import { silentRequest } from '../../utils/silent-request';

interface WebhookDto {
  readonly id: string;
  readonly organization_id: string;
  readonly project_id?: string | null;
  readonly name: string;
  readonly url: string;
  readonly events?: string[] | null;
  readonly filter_query?: string | null;
  readonly enabled: boolean;
  readonly created_at: string;
  readonly updated_at: string;
  readonly secret?: string;
}

interface DeliveryDto {
  readonly id: string;
  readonly webhook_id: string;
  readonly event: string;
  readonly payload?: unknown;
  readonly status: WebhookDeliveryStatus;
  readonly attempts?: number;
  readonly next_attempt_at?: string | null;
  readonly response_status?: number | null;
  readonly response_excerpt?: string;
  readonly duration_ms?: number;
  readonly created_at: string;
}

export function toWebhook(dto: WebhookDto): Webhook {
  return {
    id: dto.id,
    organizationId: dto.organization_id,
    projectId: dto.project_id ?? null,
    name: dto.name,
    url: dto.url,
    events: dto.events ?? [],
    filterQuery: dto.filter_query ?? '',
    enabled: dto.enabled,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

export function toDelivery(dto: DeliveryDto): WebhookDelivery {
  return {
    id: dto.id,
    webhookId: dto.webhook_id,
    event: dto.event,
    payload: dto.payload ?? null,
    status: dto.status,
    attempts: dto.attempts ?? 0,
    responseStatus: dto.response_status ?? null,
    responseExcerpt: dto.response_excerpt ?? '',
    durationMs: dto.duration_ms ?? 0,
    nextAttemptAt: dto.next_attempt_at ?? null,
    createdAt: dto.created_at,
  };
}

/** Outgoing webhooks of an organization: definitions, test ping, delivery history and secret rotation. */
@Injectable({ providedIn: 'root' })
export class WebhookApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  private path(org: string, id?: string): string {
    const base = `${this.baseUrl}/organizations/${encodeURIComponent(org)}/webhooks`;
    return id ? `${base}/${encodeURIComponent(id)}` : base;
  }

  list(org: string): Observable<readonly Webhook[]> {
    return this.http
      .get<{ data: WebhookDto[] | null }>(this.path(org), { context: silentRequest() })
      .pipe(map((body) => (body.data ?? []).map(toWebhook)));
  }

  create(org: string, draft: WebhookDraft): Observable<CreatedWebhook> {
    const body: Record<string, unknown> = {
      name: draft.name,
      url: draft.url,
      events: draft.events,
      enabled: draft.enabled,
    };
    if (draft.filterQuery.trim()) body['filter_query'] = draft.filterQuery.trim();
    if (draft.projectId) body['project_id'] = draft.projectId;
    return this.http
      .post<{ data: WebhookDto }>(this.path(org), body, { context: silentRequest() })
      .pipe(map(({ data }) => ({ webhook: toWebhook(data), secret: data.secret ?? '' })));
  }

  update(org: string, id: string, draft: Partial<WebhookDraft>): Observable<Webhook> {
    const body: Record<string, unknown> = {};
    if (draft.name !== undefined) body['name'] = draft.name;
    if (draft.url !== undefined) body['url'] = draft.url;
    if (draft.events !== undefined) body['events'] = draft.events;
    if (draft.filterQuery !== undefined) body['filter_query'] = draft.filterQuery.trim();
    if (draft.enabled !== undefined) body['enabled'] = draft.enabled;
    return this.http
      .patch<{ data: WebhookDto }>(this.path(org, id), body, { context: silentRequest() })
      .pipe(map(({ data }) => toWebhook(data)));
  }

  remove(org: string, id: string): Observable<void> {
    return this.http.delete<void>(this.path(org, id), { context: silentRequest() });
  }

  test(org: string, id: string): Observable<WebhookDelivery> {
    return this.http
      .post<{ data: DeliveryDto }>(`${this.path(org, id)}/test`, {}, { context: silentRequest() })
      .pipe(map(({ data }) => toDelivery(data)));
  }

  rotateSecret(org: string, id: string): Observable<string> {
    return this.http
      .post<{
        data: { secret: string };
      }>(`${this.path(org, id)}/rotate-secret`, {}, { context: silentRequest() })
      .pipe(map(({ data }) => data.secret));
  }

  deliveries(org: string, id: string, limit = 50): Observable<readonly WebhookDelivery[]> {
    return this.http
      .get<{ data: DeliveryDto[] | null }>(`${this.path(org, id)}/deliveries`, {
        params: new HttpParams().set('limit', limit),
        context: silentRequest(),
      })
      .pipe(map((body) => (body.data ?? []).map(toDelivery)));
  }

  redeliver(org: string, id: string, deliveryId: string): Observable<WebhookDelivery> {
    const url = `${this.path(org, id)}/deliveries/${encodeURIComponent(deliveryId)}/redeliver`;
    return this.http
      .post<{ data: DeliveryDto }>(url, {}, { context: silentRequest() })
      .pipe(map(({ data }) => toDelivery(data)));
  }
}
