/** Issue key pattern, for example CL-120. Whether a key exists is decided by the caller (resolver). */
const ISSUE_KEY_SOURCE = '\\b[A-Z][A-Z0-9]{1,9}-\\d+\\b';

export type IssueKeySegment =
  | { readonly type: 'text'; readonly value: string }
  | { readonly type: 'key'; readonly key: string };

/** Splits text into plain text and issue key segments, in order. */
export function splitIssueKeys(text: string): IssueKeySegment[] {
  const pattern = new RegExp(ISSUE_KEY_SOURCE, 'g');
  const segments: IssueKeySegment[] = [];
  let last = 0;
  for (const match of text.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > last) {
      segments.push({ type: 'text', value: text.slice(last, index) });
    }
    segments.push({ type: 'key', key: match[0] });
    last = index + match[0].length;
  }
  if (last < text.length) {
    segments.push({ type: 'text', value: text.slice(last) });
  }
  return segments;
}

/** Unique issue keys found in text, in order of first appearance. */
export function extractIssueKeys(text: string): string[] {
  const keys: string[] = [];
  for (const segment of splitIssueKeys(text)) {
    if (segment.type === 'key' && !keys.includes(segment.key)) {
      keys.push(segment.key);
    }
  }
  return keys;
}
