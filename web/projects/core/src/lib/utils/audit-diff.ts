/** One field of an audited entity, compared before and after the action. Values are plain text. */
export interface AuditDiffRow {
  readonly field: string;
  readonly before: string | null;
  readonly after: string | null;
  readonly change: 'added' | 'removed' | 'changed' | 'same';
}

function flatten(value: unknown, prefix: string, out: Map<string, string>): void {
  if (value === null || value === undefined) {
    if (prefix) out.set(prefix, 'null');
    return;
  }
  if (Array.isArray(value)) {
    if (value.length === 0 || value.every((item) => typeof item !== 'object' || item === null)) {
      out.set(prefix || '(valor)', value.map((item) => String(item)).join(', ') || '[]');
      return;
    }
    value.forEach((item, index) => flatten(item, `${prefix}[${index}]`, out));
    return;
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0 && prefix) out.set(prefix, '{}');
    for (const [key, child] of entries) flatten(child, prefix ? `${prefix}.${key}` : key, out);
    return;
  }
  out.set(prefix || '(valor)', String(value));
}

/** Key/value rows comparing the state before and after an action. Values are text, never markup. */
export function auditDiff(before: unknown, after: unknown): AuditDiffRow[] {
  const left = new Map<string, string>();
  const right = new Map<string, string>();
  flatten(before, '', left);
  flatten(after, '', right);
  const fields = [...new Set([...left.keys(), ...right.keys()])];
  return fields.map((field) => {
    const a = left.get(field) ?? null;
    const b = right.get(field) ?? null;
    const change = a === null ? 'added' : b === null ? 'removed' : a === b ? 'same' : 'changed';
    return { field, before: a, after: b, change };
  });
}
