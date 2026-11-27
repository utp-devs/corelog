import type { SearchHit } from '@corelog/core';

export type HitGroup = 'Proyectos' | 'Incidencias';

export interface HitSection {
  readonly name: HitGroup;
  readonly items: readonly SearchHit[];
}

/** Splits search hits into the Proyectos and Incidencias sections, keeping the server order. Empty sections are dropped. */
export function groupHits(hits: readonly SearchHit[]): HitSection[] {
  const projects = hits.filter((hit) => hit.kind === 'project');
  const issues = hits.filter((hit) => hit.kind === 'issue');
  const sections: HitSection[] = [];
  if (projects.length > 0) {
    sections.push({ name: 'Proyectos', items: projects });
  }
  if (issues.length > 0) {
    sections.push({ name: 'Incidencias', items: issues });
  }
  return sections;
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
