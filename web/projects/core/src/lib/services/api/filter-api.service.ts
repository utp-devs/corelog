import { HttpClient, HttpContext, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import type {
  FilterScope,
  FilterShare,
  FilterShareInput,
  FilterSubscription,
  SaveQueryPayload,
  SavedQuery,
} from '../../models/filter.model';
import type { IssueSearchPage } from '../../models/search-query.model';
import { toIssueSearchPage } from './search-query-api.service';

interface ShareDto {
  type: FilterShare['type'];
  target_id: string;
  target_name: string;
  permission: FilterShare['permission'];
}
interface SubscriptionDto {
  schedule: {
    frequency: FilterSubscription['frequency'];
    hour: number;
    weekday: number;
    timezone: string;
  };
}
interface FilterDto {
  id: string;
  name: string;
  description: string;
  query: string;
  columns: string[] | null;
  sort: string;
  owner: { id: string; name: string };
  organization_id: string;
  access: SavedQuery['access'];
  is_favorite: boolean;
  is_subscribed: boolean;
  subscription?: SubscriptionDto | null;
  shares?: ShareDto[] | null;
  updated_at: string;
}

function toShare(share: ShareDto): FilterShare {
  return {
    type: share.type,
    targetId: share.target_id,
    targetName: share.target_name,
    permission: share.permission,
  };
}

export function toSavedQuery(dto: FilterDto): SavedQuery {
  return {
    id: dto.id,
    name: dto.name,
    description: dto.description ?? '',
    query: dto.query,
    columns: dto.columns ?? [],
    sort: dto.sort ?? '',
    ownerId: dto.owner?.id ?? '',
    ownerName: dto.owner?.name ?? '',
    organizationId: dto.organization_id,
    access: dto.access,
    isFavorite: dto.is_favorite,
    isSubscribed: dto.is_subscribed,
    subscription: dto.subscription ? { ...dto.subscription.schedule } : null,
    shares: (dto.shares ?? []).map(toShare),
    updatedAt: dto.updated_at,
  };
}

/** Saved CoreQL filters: list, edit, share, run and subscribe. */
@Injectable({ providedIn: 'root' })
export class FilterApiService {
  private readonly http = inject(HttpClient);
  private readonly url = `${inject(API_URL)}/filters`;
  /** Failures are rendered by the dialogs (name taken, invalid share, ...). */
  private readonly quiet = (): HttpContext => new HttpContext().set(SKIP_ERROR_NOTIFICATION, true);

  list(scope: FilterScope = 'all', q = ''): Observable<SavedQuery[]> {
    let params = new HttpParams().set('scope', scope).set('page_size', '100');
    if (q) params = params.set('q', q);
    return this.http
      .get<{ data: FilterDto[] }>(this.url, { params, context: this.quiet() })
      .pipe(map(({ data }) => data.map(toSavedQuery)));
  }

  get(id: string): Observable<SavedQuery> {
    return this.http
      .get<{ data: FilterDto }>(`${this.url}/${id}`, { context: this.quiet() })
      .pipe(map(({ data }) => toSavedQuery(data)));
  }

  create(payload: SaveQueryPayload): Observable<SavedQuery> {
    const body = {
      name: payload.name,
      description: payload.description ?? '',
      query: payload.query,
      columns: payload.columns ?? [],
      sort: payload.sort ?? '',
      organization_id: payload.organizationId,
    };
    return this.http
      .post<{ data: FilterDto }>(this.url, body, { context: this.quiet() })
      .pipe(map(({ data }) => toSavedQuery(data)));
  }

  update(
    id: string,
    changes: Partial<Omit<SaveQueryPayload, 'organizationId'>>,
  ): Observable<SavedQuery> {
    return this.http
      .patch<{ data: FilterDto }>(`${this.url}/${id}`, changes, { context: this.quiet() })
      .pipe(map(({ data }) => toSavedQuery(data)));
  }

  delete(id: string): Observable<void> {
    return this.http.delete<void>(`${this.url}/${id}`, { context: this.quiet() });
  }

  setShares(id: string, shares: readonly FilterShareInput[]): Observable<FilterShare[]> {
    const body = {
      shares: shares.map((s) => ({
        type: s.type,
        target_id: s.targetId,
        permission: s.permission,
      })),
    };
    return this.http
      .put<{ data: ShareDto[] | { shares: ShareDto[] } }>(`${this.url}/${id}/shares`, body, {
        context: this.quiet(),
      })
      .pipe(map(({ data }) => (Array.isArray(data) ? data : data.shares).map(toShare)));
  }

  run(id: string, page = 1, pageSize = 50): Observable<IssueSearchPage> {
    const params = new HttpParams().set('page', String(page)).set('page_size', String(pageSize));
    return this.http
      .get<Parameters<typeof toIssueSearchPage>[0]>(`${this.url}/${id}/issues`, {
        params,
        context: this.quiet(),
      })
      .pipe(map(toIssueSearchPage));
  }

  subscribe(id: string, schedule: FilterSubscription): Observable<FilterSubscription> {
    return this.http
      .post<unknown>(`${this.url}/${id}/subscription`, schedule, { context: this.quiet() })
      .pipe(map(() => schedule));
  }

  unsubscribe(id: string): Observable<void> {
    return this.http.delete<void>(`${this.url}/${id}/subscription`, { context: this.quiet() });
  }
}
