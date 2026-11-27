/** Pure text edits for the Markdown editor toolbar. Each edit returns the new value and the selection to restore. */

export interface TextEdit {
  readonly value: string;
  readonly selectionStart: number;
  readonly selectionEnd: number;
}

export type ToolbarAction =
  | 'bold'
  | 'italic'
  | 'strikethrough'
  | 'code'
  | 'link'
  | 'bulletList'
  | 'numberedList'
  | 'taskList'
  | 'quote'
  | 'heading'
  | 'codeBlock'
  | 'table';

const TABLE_BLOCK = ['| Columna 1 | Columna 2 |', '| --- | --- |', '|  |  |', ''].join('\n');

function normalizeRange(
  value: string,
  selStart: number,
  selEnd: number,
): { start: number; end: number } {
  const clamp = (n: number) => Math.min(Math.max(n, 0), value.length);
  const a = clamp(selStart);
  const b = clamp(selEnd);
  return a <= b ? { start: a, end: b } : { start: b, end: a };
}

/**
 * Wraps the selection with `before` and `after`. The selected text stays selected
 * inside the markers; with an empty selection the caret lands between the markers.
 */
export function insertAtSelection(
  value: string,
  selStart: number,
  selEnd: number,
  before: string,
  after: string,
): TextEdit {
  const { start, end } = normalizeRange(value, selStart, selEnd);
  const selected = value.slice(start, end);
  const next = value.slice(0, start) + before + selected + after + value.slice(end);
  const innerStart = start + before.length;
  return { value: next, selectionStart: innerStart, selectionEnd: innerStart + selected.length };
}

/** Replaces the selection with `text` and places the caret at its end. */
function replaceSelection(value: string, selStart: number, selEnd: number, text: string): TextEdit {
  const { start, end } = normalizeRange(value, selStart, selEnd);
  const caret = start + text.length;
  return {
    value: value.slice(0, start) + text + value.slice(end),
    selectionStart: caret,
    selectionEnd: caret,
  };
}

/** Prefixes every line touched by the selection. `prefixFor` receives the zero-based line index. */
function prefixLines(
  value: string,
  selStart: number,
  selEnd: number,
  prefixFor: (index: number) => string,
): TextEdit {
  const { start, end } = normalizeRange(value, selStart, selEnd);
  const lineStart = value.lastIndexOf('\n', start - 1) + 1;
  const newlineAfter = value.indexOf('\n', end);
  const lineEnd = newlineAfter === -1 ? value.length : newlineAfter;
  const block = value
    .slice(lineStart, lineEnd)
    .split('\n')
    .map((line, index) => `${prefixFor(index)}${line}`)
    .join('\n');
  return {
    value: value.slice(0, lineStart) + block + value.slice(lineEnd),
    selectionStart: lineStart,
    selectionEnd: lineStart + block.length,
  };
}

/** Wraps the selection, or inserts a placeholder when nothing is selected, and selects the placeholder. */
function wrapOrPlaceholder(
  value: string,
  selStart: number,
  selEnd: number,
  before: string,
  after: string,
  placeholder: string,
): TextEdit {
  const { start, end } = normalizeRange(value, selStart, selEnd);
  if (start !== end) {
    return insertAtSelection(value, start, end, before, after);
  }
  const innerStart = start + before.length;
  const next = value.slice(0, start) + before + placeholder + after + value.slice(start);
  return { value: next, selectionStart: innerStart, selectionEnd: innerStart + placeholder.length };
}

/** Applies a toolbar action to the current value and selection. */
export function applyToolbarAction(
  action: ToolbarAction,
  value: string,
  selStart: number,
  selEnd: number,
): TextEdit {
  switch (action) {
    case 'bold':
      return wrapOrPlaceholder(value, selStart, selEnd, '**', '**', 'texto');
    case 'italic':
      return wrapOrPlaceholder(value, selStart, selEnd, '*', '*', 'texto');
    case 'strikethrough':
      return wrapOrPlaceholder(value, selStart, selEnd, '~~', '~~', 'texto');
    case 'code':
      return wrapOrPlaceholder(value, selStart, selEnd, '`', '`', 'código');
    case 'link':
      return wrapOrPlaceholder(value, selStart, selEnd, '[', '](https://)', 'texto');
    case 'bulletList':
      return prefixLines(value, selStart, selEnd, () => '- ');
    case 'numberedList':
      return prefixLines(value, selStart, selEnd, (index) => `${index + 1}. `);
    case 'taskList':
      return prefixLines(value, selStart, selEnd, () => '- [ ] ');
    case 'quote':
      return prefixLines(value, selStart, selEnd, () => '> ');
    case 'heading':
      return prefixLines(value, selStart, selEnd, () => '## ');
    case 'codeBlock': {
      const { start, end } = normalizeRange(value, selStart, selEnd);
      const selected = value.slice(start, end) || 'código';
      const needsBreak = start > 0 && value[start - 1] !== '\n';
      const block = `${needsBreak ? '\n' : ''}\`\`\`\n${selected}\n\`\`\`\n`;
      return replaceSelection(value, start, end, block);
    }
    case 'table': {
      const { start } = normalizeRange(value, selStart, selEnd);
      const needsBreak = start > 0 && value[start - 1] !== '\n';
      return replaceSelection(value, selStart, selEnd, `${needsBreak ? '\n' : ''}${TABLE_BLOCK}`);
    }
  }
}
