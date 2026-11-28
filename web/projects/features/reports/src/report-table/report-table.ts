import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import type { ReportCell, ReportColumn, ReportTable as ReportTableData } from '@corelog/core';

import { formatCell, sortRows, type SortDirection } from '../report-format';

/** Rows shown before "Mostrar más"; long reports (one row per issue) stay light. */
export const PAGE_ROWS = 100;

interface SortState {
  readonly index: number;
  readonly direction: SortDirection;
}

let nextId = 0;

/** Data table of a report: sortable columns, cells formatted by column type and a row limit with "Mostrar más". */
@Component({
  selector: 'app-report-table',
  templateUrl: './report-table.html',
  styleUrl: './report-table.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ReportTable {
  protected readonly captionId = `report-table-${nextId++}`;

  readonly table = input.required<ReportTableData>();
  readonly title = input.required<string>();

  protected readonly sort = signal<SortState | null>(null);
  protected readonly visible = signal(PAGE_ROWS);

  protected readonly ordered = computed(() => {
    const { rows, columns } = this.table();
    const sort = this.sort();
    return sort ? sortRows(rows, sort.index, columns[sort.index], sort.direction) : rows;
  });
  protected readonly shown = computed(() => this.ordered().slice(0, this.visible()));
  protected readonly hiddenCount = computed(() =>
    Math.max(0, this.ordered().length - this.visible()),
  );

  /** Text of a cell of the table, by the type of its column. */
  protected cell(row: readonly ReportCell[], index: number): string {
    return formatCell(row[index], this.table().columns[index].type);
  }

  protected isNumeric(column: ReportColumn): boolean {
    return column.type === 'number' || column.type === 'percent';
  }

  protected ariaSort(index: number): 'ascending' | 'descending' | 'none' {
    const sort = this.sort();
    if (sort?.index !== index) return 'none';
    return sort.direction === 'asc' ? 'ascending' : 'descending';
  }

  protected sortBy(index: number): void {
    this.sort.update((current) => {
      if (current?.index !== index) return { index, direction: 'asc' };
      return current.direction === 'asc' ? { index, direction: 'desc' } : null;
    });
  }

  protected showMore(): void {
    this.visible.update((count) => count + PAGE_ROWS);
  }
}
