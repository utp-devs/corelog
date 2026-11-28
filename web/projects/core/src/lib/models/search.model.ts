export type SearchKind = 'project' | 'issue' | 'person' | 'team' | 'filter' | 'note';

/** One result of the global search. `link` is the in-app route of the result. */
export interface SearchHit {
  readonly kind: SearchKind;
  readonly id: string;
  readonly title: string;
  readonly subtitle: string;
  readonly link: string;
  /** Project or issue key, when the result has one. */
  readonly key?: string;
  /** Title as HTML-escaped text where the matches are wrapped in mark tags. Render it only through a sanitizer. */
  readonly highlight?: string;
  /** Escaped excerpt around the match, with mark tags. */
  readonly snippet?: string;
}

/** Number of results per kind, for the whole search (not only the page returned). */
export type SearchCounts = Readonly<Partial<Record<SearchKind, number>>>;

export interface SearchResults {
  readonly hits: readonly SearchHit[];
  readonly counts: SearchCounts;
}

/** Every kind the search can return, in the order groups are shown. */
export const SEARCH_KINDS: readonly SearchKind[] = [
  'issue',
  'project',
  'person',
  'team',
  'filter',
  'note',
];
