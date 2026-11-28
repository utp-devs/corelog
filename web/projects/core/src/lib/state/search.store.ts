import { Injectable, computed, inject, signal } from '@angular/core';

import { Observable, map, tap } from 'rxjs';

import { SearchCounts, SearchHit } from '../models/search.model';
import { SearchApiService } from '../services/api/search-api.service';
import { RecentItem, getRecent, pushRecent } from '../utils/recent-items';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Shortest query sent to the search endpoint (the endpoint rejects anything shorter). */
export const SEARCH_MIN_CHARS = 2;
/** Delay between the last keystroke and the remote search of the command palette, in milliseconds. */
export const PALETTE_SEARCH_DEBOUNCE_MS = 150;

/** Global search of projects and issues, plus the recently visited items kept in localStorage. */
@Injectable({ providedIn: 'root' })
export class SearchStore {
  private readonly api = inject(SearchApiService);
  private readonly queryState = signal('');
  private readonly countsState = signal<SearchCounts>({});
  private readonly recentState = signal<RecentItem[]>(getRecent());

  readonly results = new AsyncState<SearchHit[]>();
  readonly query = this.queryState.asReadonly();
  /** Results per kind for the last search, beyond the page of hits kept. */
  readonly counts = this.countsState.asReadonly();
  readonly recent = this.recentState.asReadonly();
  readonly hits = computed<readonly SearchHit[]>(() => this.results.value() ?? []);
  readonly searchable = computed(() => this.queryState().trim().length >= SEARCH_MIN_CHARS);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Searches projects and issues. Queries shorter than SEARCH_MIN_CHARS clear the results and send nothing. */
  search(query: string, limit = 10): void {
    this.queryState.set(query);
    const term = query.trim();
    if (term.length < SEARCH_MIN_CHARS) {
      this.results.reset();
      this.countsState.set({});
      return;
    }
    const source: Observable<SearchHit[]> = this.api.searchAll(term, { limit }).pipe(
      tap((found) => this.countsState.set(found.counts)),
      map((found) => [...found.hits]),
    );
    this.results.load(source);
  }

  /** Records a visited project or issue at the top of the recent list. */
  rememberRecent(item: RecentItem): void {
    this.recentState.set(pushRecent(item));
  }

  reset(): void {
    this.queryState.set('');
    this.results.reset();
    this.countsState.set({});
  }
}
