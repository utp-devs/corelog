import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { normalizeIssueLink } from '../../config/issue-paths';
import { SearchHit } from '../../models/search.model';

interface Envelope<T> {
  data: T;
}

@Injectable({ providedIn: 'root' })
export class SearchApiService {
  private readonly http = inject(HttpClient);
  private readonly url = `${inject(API_URL)}/search`;

  search(query: string, limit = 10): Observable<SearchHit[]> {
    const params = new HttpParams().set('q', query).set('limit', String(limit));
    return this.http
      .get<Envelope<SearchHit[]>>(this.url, { params })
      .pipe(
        map((body) => body.data.map((hit) => ({ ...hit, link: normalizeIssueLink(hit.link) }))),
      );
  }
}
