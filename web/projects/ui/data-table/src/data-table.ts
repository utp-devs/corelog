import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  Directive,
  ElementRef,
  TemplateRef,
  computed,
  contentChildren,
  effect,
  inject,
  input,
  linkedSignal,
  model,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import type { MenuItem } from 'primeng/api';
import { Button } from 'primeng/button';
import { Menu } from 'primeng/menu';
import { Popover } from 'primeng/popover';
import { Select } from 'primeng/select';
import { TableModule } from 'primeng/table';

import { BulkAction, BulkBar } from '@corelog/ui/bulk-bar';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { AppTooltip } from '@corelog/ui/tooltip';

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
  /** The main action of the row: shown as a visible button next to the menu instead of inside it. */
  readonly primary?: boolean;
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

const DEFAULT_PAGE_SIZES: readonly number[] = [20, 50, 100];
const SKELETON_MAX_ROWS = 10;
const SKELETON_DEFAULT_ROWS = 5;
const SKELETON_DELAY_MS = 150;
/** Below this width the table becomes a list of cards instead of clipping its columns. */
export const CARD_LAYOUT_QUERY = '(max-width: 899.98px)';
/** Pixels of horizontal scroll left before the edge shadow disappears. */
const SCROLL_EDGE_TOLERANCE = 1;
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
  while (sibling && !sibling.classList.contains('data-table__row')) {
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
  } else if (event.key === 'Home' || event.key === 'End') {
    event.preventDefault();
    const rows = (event.currentTarget as HTMLElement).parentElement?.querySelectorAll<HTMLElement>(
      ':scope > .data-table__row',
    );
    const target = event.key === 'Home' ? rows?.[0] : rows?.[rows.length - 1];
    target?.focus();
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
  imports: [
    NgTemplateOutlet,
    FormsModule,
    Button,
    Menu,
    Popover,
    Select,
    TableModule,
    BulkBar,
    EmptyState,
    ErrorState,
    AppTooltip,
  ],
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
  /** HTTP status of the failure, used to pick friendly copy when the error message is raw. */
  readonly errorStatus = input<number | null>(null);
  /** Custom empty content (at most 280 px). Defaults to the standard empty message. */
  readonly emptyState = input<TemplateRef<void> | null>(null);
  readonly pagination = input<DataTablePagination | null>(null);
  /** Page sizes offered in the footer selector. */
  readonly pageSizes = input<readonly number[]>(DEFAULT_PAGE_SIZES);
  /** Shows the row checkboxes and the bulk bar. */
  readonly selectable = input(false);
  /** Keys of the selected rows (two-way bound). */
  readonly selectedKeys = model<string[]>([]);
  readonly bulkActions = input<BulkAction[]>([]);
  /** Makes rows focusable and emits `rowOpen` on click or Enter. */
  readonly rowOpenable = input(false);
  /** Actions of a row, shown in a menu. Rows without actions show no menu button. */
  readonly rowActions = input<((row: T) => RowMenuItem[]) | null>(null);
  /**
   * Below 900 px the table becomes a list of cards. Cards are rendered by this template; without it
   * each card lists the visible columns as label and value.
   */
  readonly mobileRender = input<TemplateRef<DataTableCellContext<T>> | null>(null);
  /** Fills the remaining height of the parent flex container. */
  readonly fill = input(false);

  readonly rowOpen = output<T>();
  readonly retry = output<void>();
  readonly pageChange = output<DataTablePageChange>();

  protected readonly cells = contentChildren(DataTableCell);

  protected readonly groupName = `corelog-density-${nextTableId++}`;
  protected readonly pageSizeOptions = computed(() =>
    this.pageSizes().map((value) => ({ label: `${value} / página`, value })),
  );
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

  private readonly isMobile = mediaQuery(CARD_LAYOUT_QUERY);
  protected readonly cardMode = computed(() => this.isMobile());
  private readonly primaryOnly = computed(
    () =>
      this.isMobile() &&
      this.mobileRender() === null &&
      this.columns().some((column) => column.mobilePrimary === true),
  );
  protected readonly showSkeleton = delayedFlag(this.loading, SKELETON_DELAY_MS);

  protected readonly hasHideable = computed(() => this.columns().some((column) => column.hideable));

  /** Roving tab stop: only one row of the table is in the tab order, arrow keys move between rows. */
  protected readonly activeRow = signal(0);

  private readonly scroller = viewChild<ElementRef<HTMLElement>>('scroller');
  protected readonly edgeStart = signal(false);
  protected readonly edgeEnd = signal(false);
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
  protected readonly range = computed(() => {
    const page = this.pagination();
    if (!page || page.total === 0) {
      return null;
    }
    const from = (page.page - 1) * page.pageSize + 1;
    return { from, to: Math.min(page.total, page.page * page.pageSize), total: page.total };
  });
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

  constructor() {
    effect((onCleanup) => {
      const element = this.scroller()?.nativeElement;
      this.data();
      this.visibleColumns();
      if (!element) {
        this.edgeStart.set(false);
        this.edgeEnd.set(false);
        return;
      }
      this.updateEdges(element);
      if (typeof ResizeObserver === 'undefined') {
        return;
      }
      const observer = new ResizeObserver(() => this.updateEdges(element));
      observer.observe(element);
      onCleanup(() => observer.disconnect());
    });
  }

  /** Shows an edge shadow on each side where there is more content to scroll to. */
  protected updateEdges(element: HTMLElement): void {
    const max = element.scrollWidth - element.clientWidth;
    this.edgeStart.set(element.scrollLeft > SCROLL_EDGE_TOLERANCE);
    this.edgeEnd.set(max - element.scrollLeft > SCROLL_EDGE_TOLERANCE);
  }

  protected rowTabIndex(index: number): number | null {
    if (!this.rowOpenable()) {
      return null;
    }
    return index === Math.min(this.activeRow(), Math.max(0, this.data().length - 1)) ? 0 : -1;
  }

  protected rowActionsOf(row: T): { primary: RowMenuItem | null; rest: RowMenuItem[] } {
    const items = this.itemsFor(row);
    const primary = items.find((item) => item.primary === true) ?? null;
    return { primary, rest: items.filter((item) => item !== primary) };
  }

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
    event.stopPropagation();
    this.rowMenuItems.set(
      this.rowActionsOf(row).rest.map((item) => ({
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
