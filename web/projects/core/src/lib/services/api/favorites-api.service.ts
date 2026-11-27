import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { FavoriteItem, RecentProjectItem } from '../../models';

/** Kinds of element that can be starred or tracked as recent. Only projects are served today. */
export type FavoriteKind = 'project' | 'filter' | 'board' | 'dashboard' | 'issue' | 'team';

interface FavoriteDto {
  type: string;
  id: string;
  name: string;
  key: string;
  color: string;
  created_at: string;
}

interface RecentDto extends FavoriteDto {
  viewed_at: string;
}

function toFavorite(dto: FavoriteDto): FavoriteItem {
  return {
    type: dto.type,
    id: dto.id,
    name: dto.name,
    key: dto.key,
    color: dto.color,
    createdAt: dto.created_at,
  };
}

function toRecent(dto: RecentDto): RecentProjectItem {
  return { ...toFavorite(dto), viewedAt: dto.viewed_at };
}

/** Stars and recently visited elements of the signed-in user. */
@Injectable({ providedIn: 'root' })
export class FavoritesApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(kind: FavoriteKind = 'project'): Observable<FavoriteItem[]> {
    const params = new HttpParams().set('type', kind);
    return this.http
      .get<{ data: FavoriteDto[] }>(`${this.baseUrl}/favorites`, { params })
      .pipe(map((response) => response.data.map(toFavorite)));
  }

  /** Idempotent. `id` is the UUID of the element. */
  star(kind: FavoriteKind, id: string): Observable<void> {
    return this.http
      .put(`${this.baseUrl}/favorites/${kind}/${encodeURIComponent(id)}`, null)
      .pipe(map(() => undefined));
  }

  /** Idempotent. Works even when the element is no longer visible. */
  unstar(kind: FavoriteKind, id: string): Observable<void> {
    return this.http
      .delete(`${this.baseUrl}/favorites/${kind}/${encodeURIComponent(id)}`)
      .pipe(map(() => undefined));
  }

  /** Most recently visited first. The server defaults to 10 and caps at 50. */
  recent(kind: FavoriteKind = 'project', limit?: number): Observable<RecentProjectItem[]> {
    let params = new HttpParams().set('type', kind);
    if (limit !== undefined) params = params.set('limit', String(limit));
    return this.http
      .get<{ data: RecentDto[] }>(`${this.baseUrl}/recent`, { params })
      .pipe(map((response) => response.data.map(toRecent)));
  }
}
