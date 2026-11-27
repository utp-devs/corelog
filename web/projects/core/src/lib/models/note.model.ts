/** Colors a note can take. The first one is the default. */
export const NOTE_COLORS = [
  'yellow',
  'blue',
  'green',
  'pink',
  'purple',
  'orange',
  'red',
  'gray',
] as const;

export type NoteColor = (typeof NOTE_COLORS)[number];

export const DEFAULT_NOTE_COLOR: NoteColor = 'yellow';

/** Spanish names of the colors, used as accessible names of the palette. */
export const NOTE_COLOR_LABELS: Readonly<Record<NoteColor, string>> = {
  yellow: 'Amarillo',
  blue: 'Azul',
  green: 'Verde',
  pink: 'Rosa',
  purple: 'Morado',
  orange: 'Naranja',
  red: 'Rojo',
  gray: 'Gris',
};

export const NOTE_TITLE_MAX = 120;
export const NOTE_BODY_MAX = 20000;
export const NOTE_TAG_MAX = 30;
export const NOTE_TAGS_MAX = 10;

/** A private note of the signed-in user. */
export interface Note {
  readonly id: string;
  readonly title: string;
  readonly body: string;
  readonly color: NoteColor;
  readonly pinned: boolean;
  readonly tags: readonly string[];
  readonly issueId: string | null;
  readonly projectId: string | null;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface NoteTagCount {
  readonly tag: string;
  readonly count: number;
}

export interface NoteFilter {
  readonly query?: string;
  readonly tag?: string;
  readonly pinned?: boolean;
  readonly issueId?: string;
  readonly projectId?: string;
  readonly cursor?: string | null;
  readonly limit?: number;
}

export interface NotePage {
  readonly items: readonly Note[];
  /** Cursor of the next page, or null on the last one. */
  readonly nextCursor: string | null;
}

export interface CreateNoteInput {
  readonly title?: string;
  readonly body?: string;
  readonly color?: NoteColor;
  readonly tags?: readonly string[];
  readonly pinned?: boolean;
  readonly issueId?: string | null;
  readonly projectId?: string | null;
}

/** Partial change: only the fields present are sent. A null link removes it. */
export interface UpdateNoteInput {
  readonly title?: string;
  readonly body?: string;
  readonly color?: NoteColor;
  readonly tags?: readonly string[];
  readonly issueId?: string | null;
  readonly projectId?: string | null;
}

/** Trims, lowercases and de-duplicates tags, dropping empty ones. Does not enforce the limits. */
export function normalizeNoteTags(tags: readonly string[]): string[] {
  const seen = new Set<string>();
  for (const raw of tags) {
    const tag = raw.trim().replace(/^#/, '').trim().toLowerCase();
    if (tag) seen.add(tag);
  }
  return [...seen];
}

/** Markdown marks that make a plain preview hard to read. */
function stripMarkdown(text: string): string {
  return text
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}(#{1,6}|>|[-*+]|\d+\.)\s+(\[[ xX]\]\s+)?/gm, '')
    .replace(/[*_`~]/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

/** Title of the note, or the start of its text when it has none. */
export function noteHeadline(note: Pick<Note, 'title' | 'body'>): string {
  if (note.title.trim()) return note.title.trim();
  const body = stripMarkdown(note.body);
  if (!body) return 'Nota sin título';
  return body.length > 60 ? `${body.slice(0, 60).trimEnd()}…` : body;
}

/** Plain text of the body for lists, cut at `max` characters. */
export function notePreview(body: string, max = 160): string {
  const plain = stripMarkdown(body);
  return plain.length > max ? `${plain.slice(0, max).trimEnd()}…` : plain;
}
