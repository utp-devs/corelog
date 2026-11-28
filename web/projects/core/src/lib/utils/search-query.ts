/** Builders and parsers for CoreQL text, the query language of the issue navigator. */

/** What the basic filters of the navigator hold. Every list is "any of"; dates are yyyy-mm-dd. */
export interface BasicFilters {
  readonly projects: readonly string[];
  readonly types: readonly string[];
  readonly statusCategories: readonly string[];
  /** Emails, `me` (the signed-in user) or `none` (unassigned). */
  readonly assignees: readonly string[];
  readonly reporters: readonly string[];
  readonly labels: readonly string[];
  readonly components: readonly string[];
  readonly fixVersions: readonly string[];
  readonly text: string;
  readonly createdFrom: string;
  readonly createdTo: string;
  readonly updatedFrom: string;
  readonly updatedTo: string;
}

export const EMPTY_BASIC_FILTERS: BasicFilters = {
  projects: [],
  types: [],
  statusCategories: [],
  assignees: [],
  reporters: [],
  labels: [],
  components: [],
  fixVersions: [],
  text: '',
  createdFrom: '',
  createdTo: '',
  updatedFrom: '',
  updatedTo: '',
};

export const ASSIGNED_TO_ME = 'me';
export const UNASSIGNED = 'none';

/** Wraps a value in double quotes, escaping quotes and backslashes. */
export function quoteValue(value: string): string {
  return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

function listClause(field: string, values: readonly string[], render = quoteValue): string | null {
  if (values.length === 0) return null;
  if (values.length === 1) return `${field} = ${render(values[0])}`;
  return `${field} IN (${values.map(render).join(', ')})`;
}

function userClause(field: string, values: readonly string[]): string | null {
  const wantsNone = values.includes(UNASSIGNED);
  const named = values.filter((value) => value !== UNASSIGNED);
  const render = (value: string): string =>
    value === ASSIGNED_TO_ME ? 'currentUser()' : quoteValue(value);
  const list = listClause(field, named, render);
  if (!wantsNone) return list;
  return list ? `(${list} OR ${field} IS EMPTY)` : `${field} IS EMPTY`;
}

/** The day after a yyyy-mm-dd date, so an inclusive end date becomes an exclusive bound. */
export function nextDay(date: string): string {
  const [year, month, day] = date.split('-').map(Number);
  return new Date(Date.UTC(year, month - 1, day + 1)).toISOString().slice(0, 10);
}

function dateClauses(field: string, from: string, to: string): string[] {
  const clauses: string[] = [];
  if (from) clauses.push(`${field} >= ${quoteValue(from)}`);
  if (to) clauses.push(`${field} < ${quoteValue(nextDay(to))}`);
  return clauses;
}

/** CoreQL text for the basic filters, without ORDER BY. No filters yields an empty text. */
export function basicFiltersToJql(filters: BasicFilters): string {
  const clauses: Array<string | null> = [
    listClause('project', filters.projects),
    listClause('type', filters.types),
    listClause('statusCategory', filters.statusCategories),
    userClause('assignee', filters.assignees),
    userClause('reporter', filters.reporters),
    listClause('labels', filters.labels),
    listClause('component', filters.components),
    listClause('fixVersion', filters.fixVersions),
    filters.text.trim() ? `text ~ ${quoteValue(filters.text.trim())}` : null,
    ...dateClauses('created', filters.createdFrom, filters.createdTo),
    ...dateClauses('updated', filters.updatedFrom, filters.updatedTo),
  ];
  return clauses.filter((clause): clause is string => clause !== null).join(' AND ');
}

export function hasBasicFilters(filters: BasicFilters): boolean {
  return basicFiltersToJql(filters) !== '';
}

/** A removable chip of the active basic filters. */
export interface QueryChip {
  readonly key: string;
  /** Filter the chip belongs to. */
  readonly field: keyof BasicFilters;
  /** The single value the chip removes, for list filters. */
  readonly value?: string;
  readonly text: string;
}

const CHIP_LABELS: Readonly<Record<string, string>> = {
  projects: 'Proyecto',
  types: 'Tipo',
  statusCategories: 'Estado',
  assignees: 'Responsable',
  reporters: 'Informante',
  labels: 'Etiqueta',
  components: 'Componente',
  fixVersions: 'Versión',
  text: 'Texto',
};

const LIST_FIELDS = [
  'projects',
  'types',
  'statusCategories',
  'assignees',
  'reporters',
  'labels',
  'components',
  'fixVersions',
] as const;

const DATE_RANGES = [
  { label: 'Creada', from: 'createdFrom', to: 'createdTo' },
  { label: 'Actualizada', from: 'updatedFrom', to: 'updatedTo' },
] as const;

/** One chip per selected value, plus the text and each date bound. `labelOf` turns a value into a name. */
export function basicFilterChips(
  filters: BasicFilters,
  labelOf: (field: string, value: string) => string = (_, value) => value,
): QueryChip[] {
  const chips: QueryChip[] = [];
  for (const field of LIST_FIELDS) {
    for (const value of filters[field]) {
      chips.push({
        key: `${field}:${value}`,
        field,
        value,
        text: `${CHIP_LABELS[field]}: ${labelOf(field, value)}`,
      });
    }
  }
  const text = filters.text.trim();
  if (text) chips.push({ key: 'text', field: 'text', text: `${CHIP_LABELS['text']}: ${text}` });
  for (const range of DATE_RANGES) {
    const from = filters[range.from];
    const to = filters[range.to];
    if (from)
      chips.push({ key: range.from, field: range.from, text: `${range.label} desde ${from}` });
    if (to) chips.push({ key: range.to, field: range.to, text: `${range.label} hasta ${to}` });
  }
  return chips;
}

/** The filters without the value (or the whole text or date) the chip stands for. */
export function removeChip(filters: BasicFilters, chip: QueryChip): BasicFilters {
  const current = filters[chip.field];
  if (Array.isArray(current)) {
    return { ...filters, [chip.field]: current.filter((value) => value !== chip.value) };
  }
  return { ...filters, [chip.field]: '' };
}

/** Splits a query at its ORDER BY clause (outside quotes). `order` is the clause without the keywords. */
export function splitOrderBy(jql: string): { readonly where: string; readonly order: string } {
  let quote = false;
  for (let index = 0; index < jql.length; index += 1) {
    const char = jql[index];
    if (char === '\\' && quote) {
      index += 1;
    } else if (char === '"') {
      quote = !quote;
    } else if (!quote && (index === 0 || /\s/.test(jql[index - 1]))) {
      const match = /^order\s+by\b\s*/i.exec(jql.slice(index));
      if (match) {
        return {
          where: jql.slice(0, index).trim(),
          order: jql.slice(index + match[0].length).trim(),
        };
      }
    }
  }
  return { where: jql.trim(), order: '' };
}

/** The query with its ORDER BY replaced, or removed when `order` is empty. */
export function withOrderBy(jql: string, order: string): string {
  const { where } = splitOrderBy(jql);
  const clause = order.trim() ? `ORDER BY ${order.trim()}` : '';
  return [where, clause].filter(Boolean).join(' ');
}

export interface SortSpec {
  readonly field: string;
  readonly direction: 'ASC' | 'DESC';
}

/** Parses `created DESC, key` into its parts. A missing direction means ascending. */
export function parseOrder(order: string): SortSpec[] {
  return order
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part): SortSpec => {
      const [field, direction] = part.split(/\s+/);
      return { field, direction: direction?.toUpperCase() === 'DESC' ? 'DESC' : 'ASC' };
    });
}

export function formatOrder(specs: readonly SortSpec[]): string {
  return specs.map((spec) => `${spec.field} ${spec.direction}`).join(', ');
}

/** What the user is writing at the cursor, to ask the right autocomplete question. */
export type CoreqlCursorKind = 'field' | 'operator' | 'value' | 'keyword' | 'order';

export interface CoreqlCursorContext {
  readonly kind: CoreqlCursorKind;
  /** Field of the clause being written (for value completions). */
  readonly field: string;
  /** Part already typed; a completion replaces it. */
  readonly prefix: string;
  readonly start: number;
  readonly end: number;
  /** The cursor is inside an unfinished double quote. */
  readonly inQuote: boolean;
}

interface Token {
  readonly type: 'word' | 'string' | 'op' | 'open' | 'close' | 'comma';
  readonly text: string;
}

const TOKEN_PATTERN = /"(?:\\.|[^"\\])*"|!=|!~|<=|>=|[=~<>]|\(|\)|,|[^\s"=~<>!(),]+/g;
const TRAILING_WORD = /[^\s"=~<>!(),]*$/;

function tokenize(text: string): Token[] {
  return Array.from(text.matchAll(TOKEN_PATTERN), (match): Token => {
    const piece = match[0];
    if (piece.startsWith('"')) return { type: 'string', text: piece };
    if (piece === '(') return { type: 'open', text: piece };
    if (piece === ')') return { type: 'close', text: piece };
    if (piece === ',') return { type: 'comma', text: piece };
    if (/^[=~<>!]/.test(piece)) return { type: 'op', text: piece };
    return { type: 'word', text: piece };
  });
}

type Phase = 'field' | 'operator' | 'value' | 'after' | 'order';

const PHASE_KIND: Readonly<Record<Phase, CoreqlCursorKind>> = {
  field: 'field',
  operator: 'operator',
  value: 'value',
  after: 'keyword',
  order: 'order',
};

/** Phase after a word or string token, given the phase before it. */
function nextPhase(phase: Phase, upper: string, text: string): { phase: Phase; field?: string } {
  switch (phase) {
    case 'field':
      if (upper === 'NOT') return { phase };
      if (upper === 'ORDER') return { phase: 'order' };
      return { phase: 'operator', field: text };
    case 'operator':
      if (upper === 'NOT') return { phase };
      return { phase: ['IN', 'IS', 'WAS', 'CHANGED'].includes(upper) ? 'value' : 'after' };
    case 'value':
      return { phase: upper === 'NOT' ? 'value' : 'after' };
    case 'after':
      if (upper === 'AND' || upper === 'OR') return { phase: 'field' };
      return { phase: upper === 'ORDER' ? 'order' : phase };
    default:
      return { phase };
  }
}

/** Works out what the user is writing at the cursor. */
export function coreqlCursorContext(text: string, cursor: number): CoreqlCursorContext {
  const head = text.slice(0, cursor);
  const inQuote = (head.match(/(?<!\\)"/g)?.length ?? 0) % 2 === 1;
  let start: number;
  let prefix: string;
  if (inQuote) {
    start = head.lastIndexOf('"');
    prefix = head.slice(start + 1);
  } else {
    prefix = TRAILING_WORD.exec(head)?.[0] ?? '';
    start = cursor - prefix.length;
  }
  let phase: Phase = 'field';
  let field = '';
  let inList = false;
  for (const token of tokenize(head.slice(0, start))) {
    if (token.type === 'word' || token.type === 'string') {
      const next = nextPhase(phase, token.text.toUpperCase(), token.text);
      phase = next.phase;
      field = next.field ?? field;
    } else if (token.type === 'op') {
      phase = 'value';
    } else if (token.type === 'open') {
      inList = inList || phase === 'value';
    } else if (token.type === 'comma') {
      if (inList) phase = 'value';
    } else {
      inList = false;
      phase = 'after';
    }
  }
  return {
    kind: inQuote ? 'value' : PHASE_KIND[phase],
    field: field.replace(/^"|"$/g, ''),
    prefix,
    start,
    end: cursor,
    inQuote,
  };
}

const PLAIN_VALUE = /^[A-Za-z0-9_.@-]+$/;

/**
 * Text of a value as it is written in a query: plain words stay as they are, everything else
 * (spaces, accents, symbols) is quoted. Functions such as `currentUser()` are kept untouched.
 */
export function valueToken(value: string, kind: string): string {
  if (kind === 'function') return value;
  return PLAIN_VALUE.test(value) ? value : quoteValue(value);
}

/** Result of accepting a completion: the new text and where the cursor goes. */
export function applyCompletion(
  text: string,
  context: CoreqlCursorContext,
  insertion: string,
  kind: string,
): { readonly text: string; readonly cursor: number } {
  const token = context.kind === 'value' ? valueToken(insertion, kind) : insertion;
  let end = context.end;
  // Inside a quote, the closing quote the user may already have is replaced too.
  if (context.inQuote && text[end] === '"') end += 1;
  const rest = text.slice(end);
  const separator = context.kind === 'value' || rest.startsWith(' ') ? '' : ' ';
  return {
    text: text.slice(0, context.start) + token + separator + rest,
    cursor: context.start + token.length + separator.length,
  };
}
