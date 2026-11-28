import type { CoreqlProblem } from '@corelog/core';

/** A piece of the query text and whether a problem covers it. */
export interface ErrorSegment {
  readonly text: string;
  readonly error: boolean;
}

/**
 * Splits the text into pieces so the ones a problem covers can be underlined. A problem with no length
 * (for example "unexpected end") underlines the last character before its position.
 */
export function errorSegments(text: string, problems: readonly CoreqlProblem[]): ErrorSegment[] {
  const flags = new Array<boolean>(text.length).fill(false);
  for (const problem of problems) {
    const start = problem.length > 0 ? problem.position : Math.max(0, problem.position - 1);
    const end = problem.length > 0 ? problem.position + problem.length : problem.position;
    for (
      let index = Math.max(0, start);
      index < Math.min(text.length, Math.max(end, start + 1));
      index += 1
    ) {
      flags[index] = true;
    }
  }
  const segments: ErrorSegment[] = [];
  for (let index = 0; index < text.length; index += 1) {
    const last = segments[segments.length - 1];
    if (last && last.error === flags[index]) {
      segments[segments.length - 1] = { text: last.text + text[index], error: last.error };
    } else {
      segments.push({ text: text[index], error: flags[index] });
    }
  }
  return segments;
}

/** The text with the range [position, position + length) replaced, and where the cursor goes. */
export function replaceRange(
  text: string,
  position: number,
  length: number,
  replacement: string,
): { readonly text: string; readonly cursor: number } {
  return {
    text: text.slice(0, position) + replacement + text.slice(position + length),
    cursor: position + replacement.length,
  };
}
