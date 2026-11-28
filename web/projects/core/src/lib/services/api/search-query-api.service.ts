import { HttpClient, HttpContext, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION, SKIP_LOADING } from '../../config/http-context';
import type {
  CoreqlSuggestions,
  CoreqlValidation,
  IssueSearchPage,
  IssueSearchParams,
  SearchedIssue,
} from '../../models/search-query.model';
import { toIssue } from './issue-api.service';

type IssueDto = Parameters<typeof toIssue>[0];

interface SearchedIssueDto extends IssueDto {
  readonly labels?: string[];
  readonly vote_count?: number;
}

interface PageDto {
  readonly data: SearchedIssueDto[];
  readonly meta: {
    readonly page: number;
    readonly page_size: number;
    readonly total?: number;
    readonly query?: string;
    readonly warnings?: Array<{ code: string; message: string }>;
  };
}

/** Maps one row of the search to the client model. */
export function toSearchedIssue(dto: SearchedIssueDto): SearchedIssue {
  return { ...toIssue(dto), labels: dto.labels ?? [], voteCount: dto.vote_count ?? 0 };
}

/** Maps a page of the search or of a saved filter run. */
export function toIssueSearchPage(body: PageDto): IssueSearchPage {
  return {
    items: body.data.map(toSearchedIssue),
    total: body.meta.total ?? body.data.length,
    page: body.meta.page,
    pageSize: body.meta.page_size,
    query: body.meta.query ?? '',
    warnings: body.meta.warnings ?? [],
  };
}

/** CoreQL: running a query, validating it as the user types and suggesting what comes next. */
@Injectable({ providedIn: 'root' })
export class SearchQueryApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);
  /** Errors of the query are shown next to the editor, and typing must not move the loading bar. */
  private readonly quiet = (): HttpContext =>
    new HttpContext().set(SKIP_ERROR_NOTIFICATION, true).set(SKIP_LOADING, true);

  searchIssues(params: IssueSearchParams): Observable<IssueSearchPage> {
    let query = new HttpParams().set('jql', params.jql);
    if (params.order) query = query.set('order', params.order);
    if (params.page) query = query.set('page', String(params.page));
    if (params.pageSize) query = query.set('page_size', String(params.pageSize));
    return this.http
      .get<PageDto>(`${this.baseUrl}/issues/search`, {
        params: query,
        context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true),
      })
      .pipe(map(toIssueSearchPage));
  }

  validate(jql: string): Observable<CoreqlValidation> {
    return this.http
      .post<{
        data: { valid: boolean; query: string; errors: CoreqlValidation['errors'] };
      }>(`${this.baseUrl}/search/validate`, { jql }, { context: this.quiet() })
      .pipe(
        map(({ data }) => ({ valid: data.valid, query: data.query, errors: data.errors ?? [] })),
      );
  }

  /** Completions of the query at the cursor. */
  complete(jql: string, cursor: number, limit = 10): Observable<CoreqlSuggestions> {
    const params = new HttpParams()
      .set('jql', jql)
      .set('cursor', String(cursor))
      .set('limit', String(limit));
    return this.suggest(params);
  }

  /** Values of one field that start with the prefix, limited to what the user can see. */
  values(field: string, prefix: string, limit = 10): Observable<CoreqlSuggestions> {
    const params = new HttpParams()
      .set('field', field)
      .set('q', prefix)
      .set('limit', String(limit));
    return this.suggest(params);
  }

  private suggest(params: HttpParams): Observable<CoreqlSuggestions> {
    return this.http
      .get<{ data: Partial<CoreqlSuggestions> }>(`${this.baseUrl}/search/suggest`, {
        params,
        context: this.quiet(),
      })
      .pipe(
        map(({ data }) => ({ completions: data.completions ?? [], values: data.values ?? [] })),
      );
  }
}
