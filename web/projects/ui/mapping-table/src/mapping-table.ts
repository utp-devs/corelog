import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';

import {
  IMPORT_FIELDS,
  assignField,
  fieldByColumn,
  missingRequired,
  type FieldMapping,
  type MappingField,
} from './mapping';

/**
 * Table that maps the columns of an uploaded file to CoreLog fields: one row per column with its sample values and
 * a field select. A field can be used by one column only (used fields are disabled in the other rows) and the
 * required fields are flagged until they have a column.
 */
@Component({
  selector: 'app-mapping-table',
  templateUrl: './mapping-table.html',
  styleUrl: './mapping-table.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MappingTable {
  readonly columns = input.required<readonly string[]>();
  /** Rows of the file used to show example values. */
  readonly sampleRows = input<readonly (readonly string[])[]>([]);
  readonly fields = input<readonly MappingField[]>(IMPORT_FIELDS);
  /** CoreLog field -> CSV column. */
  readonly mapping = model<FieldMapping>({});

  protected readonly byColumn = computed(() => fieldByColumn(this.mapping()));
  protected readonly missing = computed(() => missingRequired(this.mapping(), this.fields()));
  protected readonly rows = computed(() =>
    this.columns().map((name, index) => ({
      name,
      samples: this.sampleRows()
        .map((row) => row[index] ?? '')
        .filter((value) => value !== '')
        .slice(0, 3),
    })),
  );

  protected fieldOf(column: string): string {
    return this.byColumn()[column] ?? '';
  }

  protected isUsedElsewhere(field: string, column: string): boolean {
    const col = this.mapping()[field];
    return !!col && col !== column;
  }

  protected labelOf(field: string): string {
    return this.fields().find((f) => f.key === field)?.label ?? '';
  }

  protected choose(column: string, event: Event): void {
    const field = (event.target as HTMLSelectElement).value;
    this.mapping.set(assignField(this.mapping(), column, field));
  }
}
