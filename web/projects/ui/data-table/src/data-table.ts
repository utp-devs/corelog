import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  Directive,
  TemplateRef,
  computed,
  contentChildren,
  inject,
  input,
  linkedSignal,
  model,
  output,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import type { MenuItem } from 'primeng/api';
import { Button } from 'primeng/button';
import { Menu } from 'primeng/menu';
import { Popover } from 'primeng/popover';
import { Select } from 'primeng/select';
import { TableModule } from 'primeng/table';

import { BulkAction, BulkBar } from '@corelog/ui/bulk-bar';

import {
  DENSITY_STORAGE_KEY,
  delayedFlag,
  mediaQuery,
  persistentState,
  readStoredValue,
  writeStoredValue,
} from './table-state';

export type Density = 'comfortable' | 'compact';
export type ColumnAlign = 'left' | 'center' | 'right';

/** Column definition. `key` identifies the column when its visibility is persisted. */
export interface DataTableColumn {
  readonly key: string;
  readonly title: string;
  /** Property read from each row when no cell template is given for the column. */
  readonly field?: string;
  readonly align?: ColumnAlign;
  /** Truncates long values and exposes the full value as the cell title. */
  readonly ellipsis?: boolean;
  /** Adds sorting by this column over the visible rows. */
  readonly sortable?: boolean;
  /** Listed in the column chooser. */
  readonly hideable?: boolean;
  /** Hidden by default, unless the user has saved their own choice. */
  readonly defaultHidden?: boolean;
  /** On mobile, when no card template is set, only the columns marked as primary are shown. */
  readonly mobilePrimary?: boolean;
}

/** Row action, shown in the row menu. */
export interface RowMenuItem {
  readonly key: string;
  readonly label: string;
  readonly icon?: string;
  readonly danger?: boolean;
  readonly onClick: () => void;
}

export interface DataTablePagination {
  /** Current page, starts at 1. */
  readonly page: number;
  readonly pageSize: number;
  readonly total: number;
}

/** Emitted when the page or the page size changes. Pagination is server side. */
export interface DataTablePageChange {
  readonly page: number;
  readonly pageSize: number;
}

export interface DataTableCellContext<T> {
  readonly $implicit: T;
}

/**
 * Custom cell content for one column. Place it as `<ng-template appDataTableCell="key" let-row>`
 * inside the table element.
 */
@Directive({ selector: '[appDataTableCell]' })
export class DataTableCell<T = unknown> {
  readonly column = input.required<string>({ alias: 'appDataTableCell' });
  readonly template = inject<TemplateRef<DataTableCellContext<T>>>(TemplateRef);
}

const PAGE_SIZE_OPTIONS = [20, 50, 100].map((value) => ({ label: `${value} / página`, value }));
const SKELETON_MAX_ROWS = 10;
const SKELETON_DEFAULT_ROWS = 5;
const SKELETON_DELAY_MS = 150;
const MOBILE_QUERY = '(max-width: 767px)';
const INTERACTIVE_SELECTOR = 'button, a, input, label, select, textarea, [role="menuitem"]';

let nextTableId = 0;

function isDensity(value: unknown): value is Density {
  return value === 'comfortable' || value === 'compact';
}

function isInteractiveTarget(target: EventTarget | null): boolean {
  return target instanceof Element && target.closest(INTERACTIVE_SELECTOR) !== null;
}

function moveRowFocus(current: HTMLElement, step: 1 | -1): void {
  let sibling = step === 1 ? current.nextElementSibling : current.previousElementSibling;
  while (sibling && !sibling.classList.contains('cl-data-row')) {
    sibling = step === 1 ? sibling.nextElementSibling : sibling.previousElementSibling;
  }
  if (sibling instanceof HTMLElement) {
    sibling.focus();
  }
}

/** Keyboard on a focused row: arrow up and down move focus, Enter opens the row. */
function handleRowKeyDown(event: KeyboardEvent, onOpen: () => void): void {
  if (event.target !== event.currentTarget) {
    return;
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    moveRowFocus(event.currentTarget as HTMLElement, event.key === 'ArrowDown' ? 1 : -1);
  } else if (event.key === 'Enter') {
    event.preventDefault();
    onOpen();
  }
}

function cellValue(row: object, field: string): unknown {
  return (row as Record<string, unknown>)[field];
}

/** Orders two cell values ascending. Empty values always sort last. */
function compareCells(a: unknown, b: unknown): number {
  if (a == null || b == null) {
    return a == null && b == null ? 0 : a == null ? 1 : -1;
  }
  if (typeof a === 'number' && typeof b === 'number') {
    return a - b;
  }
  return String(a).localeCompare(String(b), 'es', { numeric: true, sensitivity: 'base' });
}

/**
 * Table over PrimeNG p-table: sorting, selection with BulkBar, loading skeleton after a delay,
 * error and empty states, server pagination, mobile card layout, persisted density and
 * column visibility, row actions and keyboard navigation.
 */
@Component({
  selector: 'app-data-table',
  imports: [NgTemplateOutlet, FormsModule, Button, Menu, Popover, Select, TableModule, BulkBar],
  templateUrl: './data-table.html',
  styleUrl: './data-table.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '[class.fill-host]': 'fill()',
  },
})
export class DataTable<T extends object> {
  /** Key of the column-visibility preference (`corelog.table.<tableKey>.columns`). */
  readonly tableKey = input.required<string>();
  readonly columns = input.required<DataTableColumn[]>();
  readonly data = input.required<T[]>();
  /** Name of the identifying property, or a function that returns the row key. */
  readonly rowKey = input.required<string | ((row: T) => string)>();
  readonly loading = input(false);
  /** Error message: replaces the table with an error state and a retry button. */
  readonly error = input<string | null>(null);
  /** Custom empty content (at most 280 px). Defaults to the standard empty message. */
  readonly emptyState = input<TemplateRef<void> | null>(null);
  readonly pagination = input<DataTablePagination | null>(null);
  /** Shows the row checkboxes and the bulk bar. */
  readonly selectable = input(false);
  /** Keys of the selected rows (two-way bound). */
  readonly selectedKeys = model<string[]>([]);
  readonly bulkActions = input<BulkAction[]>([]);
  /** Makes rows focusable and emits `rowOpen` on click or Enter. */
  readonly rowOpenable = input(false);
  /** Actions of a row, shown in a menu. Rows without actions show no menu button. */
  readonly rowActions = input<((row: T) => RowMenuItem[]) | null>(null);
  /** Below 768 px replaces the table with a list of cards rendered by this template. */
  readonly mobileRender = input<TemplateRef<DataTableCellContext<T>> | null>(null);
  /** Fills the remaining height of the parent flex container. */
  readonly fill = input(false);

  readonly rowOpen = output<T>();
  readonly retry = output<void>();
  readonly pageChange = output<DataTablePageChange>();

  protected readonly cells = contentChildren(DataTableCell);

  protected readonly groupName = `corelog-density-${nextTableId++}`;
  protected readonly pageSizeOptions = PAGE_SIZE_OPTIONS;
  protected readonly rowMenuItems = signal<MenuItem[]>([]);

  protected readonly density = persistentState<Density>(DENSITY_STORAGE_KEY, 'comfortable');
  protected readonly activeDensity = computed<Density>(() => {
    const stored = this.density.value();
    return isDensity(stored) ? stored : 'comfortable';
  });

  private readonly defaultHidden = computed(() =>
    this.columns()
      .filter((column) => column.defaultHidden)
      .map((column) => column.key),
  );
  private readonly columnsStorageKey = computed(() => `corelog.table.${this.tableKey()}.columns`);
  private readonly hiddenKeys = linkedSignal<string[]>(() =>
    readStoredValue(this.columnsStorageKey(), this.defaultHidden()),
  );

  private readonly isMobile = mediaQuery(MOBILE_QUERY);
  protected readonly cardMode = computed(() => this.isMobile() && this.mobileRender() !== null);
  private readonly primaryOnly = computed(
    () =>
      this.isMobile() &&
      !this.cardMode() &&
      this.columns().some((column) => column.mobilePrimary === true),
  );
  protected readonly showSkeleton = delayedFlag(this.loading, SKELETON_DELAY_MS);

  protected readonly hasHideable = computed(() => this.columns().some((column) => column.hideable));
  protected readonly visibleColumns = computed(() => {
    const hidden = this.hiddenKeys();
    return this.columns().filter((column) => {
      if (this.primaryOnly()) {
        return column.mobilePrimary === true;
      }
      return !(column.hideable && hidden.includes(column.key));
    });
  });

  protected readonly skeletonRows = computed(() => {
    const count = Math.min(this.pagination()?.pageSize ?? SKELETON_DEFAULT_ROWS, SKELETON_MAX_ROWS);
    return Array.from({ length: count }, (_, index) => ({ index }));
  });

  protected readonly sort = signal<{ key: string; direction: 'asc' | 'desc' } | null>(null);
  protected readonly rows = computed<T[]>(() => {
    const data = this.data();
    const sort = this.sort();
    const column = sort ? this.columns().find((item) => item.key === sort.key) : undefined;
    if (!sort || !column?.sortable) {
      return data;
    }
    const field = column.field ?? column.key;
    const factor = sort.direction === 'asc' ? 1 : -1;
    return [...data].sort((a, b) => {
      const valueA = cellValue(a, field);
      const valueB = cellValue(b, field);
      if (valueA == null || valueB == null) {
        return compareCells(valueA, valueB);
      }
      return factor * compareCells(valueA, valueB);
    });
  });

  protected readonly anyRowActions = computed(() => {
    const actions = this.rowActions();
    return actions !== null && this.data().some((row) => actions(row).length > 0);
  });
  protected readonly showActionsColumn = computed(() => this.anyRowActions() && !this.cardMode());
  protected readonly colspan = computed(
    () =>
      this.visibleColumns().length +
      (this.selectable() ? 1 : 0) +
      (this.showActionsColumn() ? 1 : 0),
  );
  protected readonly totalPages = computed(() => {
    const page = this.pagination();
    return page ? Math.max(1, Math.ceil(page.total / page.pageSize)) : 1;
  });

  protected readonly allChecked = computed(() => {
    const selected = this.selectedKeys();
    const rows = this.data();
    return rows.length > 0 && rows.every((row) => selected.includes(this.keyOf(row)));
  });
  protected readonly someChecked = computed(() => {
    const selected = this.selectedKeys();
    return !this.allChecked() && this.data().some((row) => selected.includes(this.keyOf(row)));
  });

  protected keyOf(row: T): string {
    const key = this.rowKey();
    return typeof key === 'function' ? key(row) : String((row as Record<string, unknown>)[key]);
  }

  protected cellText(row: T, column: DataTableColumn): string {
    if (!column.field) {
      return '';
    }
    const value = cellValue(row, column.field);
    return value == null ? '' : String(value);
  }

  protected templateFor(key: string): TemplateRef<DataTableCellContext<unknown>> | null {
    return this.cells().find((cell) => cell.column() === key)?.template ?? null;
  }

  protected ariaSort(column: DataTableColumn): string {
    const sort = this.sort();
    if (sort?.key !== column.key) {
      return 'none';
    }
    return sort.direction === 'asc' ? 'ascending' : 'descending';
  }

  protected toggleSort(key: string): void {
    const sort = this.sort();
    if (sort?.key !== key) {
      this.sort.set({ key, direction: 'asc' });
    } else if (sort.direction === 'asc') {
      this.sort.set({ key, direction: 'desc' });
    } else {
      this.sort.set(null);
    }
  }

  protected setDensity(value: string): void {
    this.density.set(isDensity(value) ? value : 'comfortable');
  }

  protected onColumnToggle(key: string, event: Event): void {
    this.setColumnVisible(key, (event.target as HTMLInputElement).checked);
  }

  protected setColumnVisible(key: string, visible: boolean): void {
    const hidden = this.hiddenKeys();
    const next = visible ? hidden.filter((item) => item !== key) : [...hidden, key];
    this.hiddenKeys.set(next);
    writeStoredValue(this.columnsStorageKey(), next);
  }

  protected isColumnVisible(key: string): boolean {
    return !this.hiddenKeys().includes(key);
  }

  protected isSelected(row: T): boolean {
    return this.selectedKeys().includes(this.keyOf(row));
  }

  protected toggleRow(row: T, event: Event): void {
    const checked = (event.target as HTMLInputElement).checked;
    const key = this.keyOf(row);
    const current = this.selectedKeys();
    this.selectedKeys.set(checked ? [...current, key] : current.filter((item) => item !== key));
  }

  protected toggleAll(event: Event): void {
    const checked = (event.target as HTMLInputElement).checked;
    const keys = this.data().map((row) => this.keyOf(row));
    const current = this.selectedKeys();
    this.selectedKeys.set(
      checked
        ? [...current, ...keys.filter((key) => !current.includes(key))]
        : current.filter((key) => !keys.includes(key)),
    );
  }

  protected onRowClick(event: MouseEvent, row: T): void {
    if (this.rowOpenable() && !isInteractiveTarget(event.target)) {
      this.rowOpen.emit(row);
    }
  }

  protected onRowKeyDown(event: KeyboardEvent, row: T): void {
    if (this.rowOpenable()) {
      handleRowKeyDown(event, () => this.rowOpen.emit(row));
    }
  }

  protected itemsFor(row: T): RowMenuItem[] {
    return this.rowActions()?.(row) ?? [];
  }

  protected openRowMenu(event: Event, row: T, menu: Menu): void {
    this.rowMenuItems.set(
      this.itemsFor(row).map((item) => ({
        label: item.label,
        icon: item.icon,
        styleClass: item.danger ? 'row-action-danger' : undefined,
        command: () => item.onClick(),
      })),
    );
    menu.toggle(event);
  }

  protected changePage(page: number, pageSize: number): void {
    this.pageChange.emit({ page, pageSize });
  }

  protected onPageSizeChange(pageSize: number | null): void {
    const page = this.pagination();
    if (page && pageSize !== null) {
      this.pageChange.emit({ page: page.page, pageSize });
    }
  }
}
