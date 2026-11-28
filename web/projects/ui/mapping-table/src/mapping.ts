/** A CoreLog field an imported column can fill. */
export interface MappingField {
  /** Key the API uses in the mapping. */
  readonly key: string;
  readonly label: string;
  readonly required?: boolean;
  readonly hint?: string;
}

/** Fields of an issue the CSV import understands, in the order they are offered. */
export const IMPORT_FIELDS: readonly MappingField[] = [
  { key: 'summary', label: 'Resumen', required: true, hint: 'El título de la incidencia' },
  { key: 'description', label: 'Descripción' },
  { key: 'type', label: 'Tipo de incidencia' },
  { key: 'priority', label: 'Prioridad' },
  { key: 'status', label: 'Estado' },
  { key: 'assignee', label: 'Responsable', hint: 'Correo de una persona de la organización' },
  { key: 'reporter', label: 'Informador', hint: 'Correo de una persona de la organización' },
  { key: 'labels', label: 'Etiquetas', hint: 'Separadas por coma' },
  { key: 'components', label: 'Componentes' },
  { key: 'fix_versions', label: 'Versiones' },
  { key: 'due_date', label: 'Fecha de vencimiento' },
  { key: 'start_date', label: 'Fecha de inicio' },
  { key: 'story_points', label: 'Puntos de historia' },
  { key: 'parent', label: 'Incidencia padre' },
];

/** CoreLog field -> CSV column, as the API stores it. */
export type FieldMapping = Readonly<Record<string, string>>;

/** CSV column -> CoreLog field, the way the table shows it (one field per column). */
export function fieldByColumn(mapping: FieldMapping): Readonly<Record<string, string>> {
  const out: Record<string, string> = {};
  for (const [field, column] of Object.entries(mapping)) {
    if (column) out[column] = field;
  }
  return out;
}

/**
 * Assigns a field to a column (or none with an empty field). A field is filled by one column only, so a previous
 * column of that field is released; the previous field of the column is released too.
 */
export function assignField(mapping: FieldMapping, column: string, field: string): FieldMapping {
  const next: Record<string, string> = {};
  for (const [key, col] of Object.entries(mapping)) {
    if (col === column || key === field) continue;
    next[key] = col;
  }
  if (field) next[field] = column;
  return next;
}

/** Required fields without a column. */
export function missingRequired(
  mapping: FieldMapping,
  fields: readonly MappingField[] = IMPORT_FIELDS,
): readonly MappingField[] {
  return fields.filter((f) => f.required && !mapping[f.key]);
}

/** Keeps only the entries whose column exists in the file. */
export function pruneMapping(mapping: FieldMapping, columns: readonly string[]): FieldMapping {
  const known = new Set(columns);
  return Object.fromEntries(Object.entries(mapping).filter(([, column]) => known.has(column)));
}
