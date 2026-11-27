import { NOTE_TAGS_MAX, NOTE_TAG_MAX, normalizeNoteTags } from '@corelog/core';

export interface TagAddResult {
  /** Tags after adding what was valid. */
  readonly tags: string[];
  /** Spanish message for the first rejected entry, or null when every entry was accepted. */
  readonly error: string | null;
}

export const TAG_TOO_LONG = `Cada etiqueta puede tener hasta ${NOTE_TAG_MAX} caracteres.`;
export const TOO_MANY_TAGS = `Una nota admite hasta ${NOTE_TAGS_MAX} etiquetas.`;

/**
 * Adds the tags typed in a field (separated by commas) to the current ones. Tags are trimmed,
 * lowercased and never repeated; entries over the length or count limit are rejected with a message.
 */
export function addNoteTags(current: readonly string[], typed: string): TagAddResult {
  const tags = [...current];
  let error: string | null = null;
  for (const tag of normalizeNoteTags(typed.split(','))) {
    if (tags.includes(tag)) continue;
    if (tag.length > NOTE_TAG_MAX) {
      error ??= TAG_TOO_LONG;
    } else if (tags.length >= NOTE_TAGS_MAX) {
      error ??= TOO_MANY_TAGS;
    } else {
      tags.push(tag);
    }
  }
  return { tags, error };
}
