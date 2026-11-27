export type SearchKind = 'project' | 'issue';

/** One result of the global search. `link` is the in-app route of the project or issue. */
export interface SearchHit {
  readonly kind: SearchKind;
  readonly id: string;
  readonly title: string;
  readonly subtitle: string;
  readonly link: string;
}
