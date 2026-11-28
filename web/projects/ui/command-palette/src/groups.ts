import type { SearchHit } from '@corelog/core';

export type HitGroup = 'Proyectos' | 'Incidencias' | 'Personas' | 'Equipos' | 'Filtros' | 'Notas';

export interface HitSection {
  readonly name: HitGroup;
  readonly items: readonly SearchHit[];
}

const GROUPS: ReadonlyArray<{ kind: SearchHit['kind']; name: HitGroup }> = [
  { kind: 'project', name: 'Proyectos' },
  { kind: 'issue', name: 'Incidencias' },
  { kind: 'person', name: 'Personas' },
  { kind: 'team', name: 'Equipos' },
  { kind: 'filter', name: 'Filtros' },
  { kind: 'note', name: 'Notas' },
];

/** Splits search hits into one section per kind, keeping the server order. Empty sections are dropped. */
export function groupHits(hits: readonly SearchHit[]): HitSection[] {
  return GROUPS.map(({ kind, name }) => ({
    name,
    items: hits.filter((hit) => hit.kind === kind),
  })).filter((section) => section.items.length > 0);
}

/** Lower-case, accent-free form used for matching. */
export function normalizeText(value: string): string {
  return value.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();
}

/** Every whitespace-separated word of the query must appear in the text (order free). An empty query matches everything. */
export function matchesQuery(text: string, query: string): boolean {
  const words = normalizeText(query).split(/\s+/).filter(Boolean);
  if (words.length === 0) {
    return true;
  }
  const haystack = normalizeText(text);
  return words.every((word) => haystack.includes(word));
}

export interface HighlightSegment {
  readonly text: string;
  readonly match: boolean;
}

/**
 * Splits text into segments so the words of the query can be highlighted. Matching ignores case and
 * accents, like `matchesQuery`, but the segments keep the original characters.
 */
export function highlightSegments(text: string, query: string): HighlightSegment[] {
  const words = normalizeText(query).split(/\s+/).filter(Boolean);
  if (words.length === 0 || text === '') {
    return [{ text, match: false }];
  }

  // Normalized text with, for each of its characters, the index of the original character it came from.
  let normalized = '';
  const origin: number[] = [];
  Array.from(text).reduce((offset, char) => {
    const folded = char.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
    for (let i = 0; i < folded.length; i++) origin.push(offset);
    normalized += folded;
    return offset + char.length;
  }, 0);

  const marked = new Array<boolean>(text.length).fill(false);
  for (const word of words) {
    for (
      let from = normalized.indexOf(word);
      from !== -1;
      from = normalized.indexOf(word, from + 1)
    ) {
      const start = origin[from];
      const end = origin[from + word.length - 1] + 1;
      for (let i = start; i < end; i++) marked[i] = true;
    }
  }

  const segments: HighlightSegment[] = [];
  let begin = 0;
  for (let i = 1; i <= text.length; i++) {
    if (i === text.length || marked[i] !== marked[begin]) {
      segments.push({ text: text.slice(begin, i), match: marked[begin] });
      begin = i;
    }
  }
  return segments;
}
