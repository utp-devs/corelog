import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { ISSUES_PATH, normalizeIssueLink } from '../../config/issue-paths';
import { SearchHit, SearchKind, SearchResults } from '../../models/search.model';

interface SearchBody {
  data: SearchHit[];
  meta?: { counts?: Partial<Record<SearchKind, number>> };
}

/**
 * Router path of a result. The server links filters and notes to addresses the app does not have
 * (/filters/:id, /notes/:id), so those open the navigator with the filter and the notes list.
 */
export function searchHitLink(hit: Pick<SearchHit, 'kind' | 'id' | 'link'>): string {
  switch (hit.kind) {
    case 'filter':
      return `${ISSUES_PATH}?filter=${hit.id}`;
    case 'note':
      return '/notes';
    default:
      return normalizeIssueLink(hit.link);
  }
}

@Injectable({ providedIn: 'root' })
export class SearchApiService {
  private readonly http = inject(HttpClient);
  private readonly url = `${inject(API_URL)}/search`;

  search(query: string, limit = 10): Observable<SearchHit[]> {
    return this.searchAll(query, { limit }).pipe(map((results) => [...results.hits]));
  }

  /** Searches the chosen kinds (all by default) and returns the hits with the counts per kind. */
  searchAll(
    query: string,
    options: { limit?: number; types?: readonly SearchKind[] } = {},
  ): Observable<SearchResults> {
    let params = new HttpParams().set('q', query).set('limit', String(options.limit ?? 10));
    params = params.set('types', options.types?.join(',') ?? 'all');
    return this.http.get<SearchBody>(this.url, { params }).pipe(
      map((body) => ({
        hits: body.data.map((hit) => ({ ...hit, link: searchHitLink(hit) })),
        counts: body.meta?.counts ?? {},
      })),
    );
  }
}
