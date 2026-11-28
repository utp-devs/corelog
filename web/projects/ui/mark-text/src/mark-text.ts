import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

/** A piece of text and whether it is a match. */
export interface MarkSegment {
  readonly text: string;
  readonly mark: boolean;
}

const MARK_TAG = /<(\/?)mark>/gi;
const ENTITIES: Readonly<Record<string, string>> = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#39;': "'",
  '&#x27;': "'",
};

function decode(text: string): string {
  return text.replace(/&(?:amp|lt|gt|quot|#39|#x27);/g, (entity) => ENTITIES[entity] ?? entity);
}

/**
 * Splits the highlighted text of a search result into plain and matched pieces. The server sends the
 * text with its HTML escaped and the matches between mark tags: only those tags are understood, and
 * everything else is plain text, so no markup other than a highlight can ever come out of it.
 */
export function markSegments(highlighted: string | null | undefined): MarkSegment[] {
  if (!highlighted) return [];
  const segments: MarkSegment[] = [];
  let marked = false;
  let last = 0;
  const push = (end: number): void => {
    const text = decode(highlighted.slice(last, end));
    if (text) segments.push({ text, mark: marked });
  };
  for (const match of highlighted.matchAll(MARK_TAG)) {
    push(match.index);
    marked = match[1] === '';
    last = match.index + match[0].length;
  }
  push(highlighted.length);
  return segments;
}

/**
 * Text with the matches of a search highlighted. Renders text nodes and mark elements only (no innerHTML),
 * so unexpected markup in the input is shown as text instead of being executed.
 */
@Component({
  selector: 'app-mark-text',
  templateUrl: './mark-text.html',
  styleUrl: './mark-text.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MarkText {
  /** Text with its HTML escaped and the matches wrapped in mark tags. */
  readonly highlighted = input<string | null | undefined>('');
  /** Shown when there is no highlighted text. */
  readonly fallback = input('');

  protected readonly segments = computed<MarkSegment[]>(() => {
    const parts = markSegments(this.highlighted());
    return parts.length > 0 ? parts : [{ text: this.fallback(), mark: false }];
  });
}
