import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import type { BulkAction } from '@corelog/ui/bulk-bar';

import { DataTable, DataTableColumn, DataTablePagination, RowMenuItem } from './data-table';

type Row = { id: string; title: string; createdAt: string };

const rows: Row[] = [
  { id: '1', title: 'Fila 1', createdAt: '2026-11-01' },
  { id: '2', title: 'Fila 2', createdAt: '2026-11-02' },
  { id: '3', title: 'Fila 3', createdAt: '2026-11-03' },
];

const columns: DataTableColumn[] = [
  { key: 'title', title: 'Título', field: 'title', mobilePrimary: true },
  { key: 'createdAt', title: 'Creado', field: 'createdAt', hideable: true, align: 'right' },
];

@Component({
  imports: [DataTable],
  template: `
    <ng-template #mobile let-row
      ><span>Tarjeta {{ row.title }}</span></ng-template
    >
    <ng-template #empty><p>Aún no hay filas</p></ng-template>
    <app-data-table
      [tableKey]="tableKey()"
      [columns]="columns()"
      [data]="data()"
      rowKey="id"
      [loading]="loading()"
      [error]="error()"
      [pagination]="pagination()"
      [selectable]="selectable()"
      [(selectedKeys)]="selected"
      [bulkActions]="bulkActions()"
      [rowOpenable]="rowOpenable()"
      [rowActions]="rowActions()"
      [mobileRender]="useMobile() ? mobile : null"
      [emptyState]="useEmpty() ? empty : null"
      [fill]="fill()"
      (retry)="onRetry()"
      (rowOpen)="onOpen($event)"
      (pageChange)="onPage($event)"
    />
  `,
})
class DataTableHost {
  readonly tableKey = signal('test-table');
  readonly columns = signal<DataTableColumn[]>(columns);
  readonly data = signal<Row[]>(rows);
  readonly loading = signal(false);
  readonly error = signal<string | null>(null);
  readonly pagination = signal<DataTablePagination | null>(null);
  readonly selectable = signal(false);
  readonly selected = signal<string[]>([]);
  readonly bulkActions = signal<BulkAction[]>([]);
  readonly rowOpenable = signal(false);
  readonly rowActions = signal<((row: Row) => RowMenuItem[]) | null>(null);
  readonly useMobile = signal(false);
  readonly useEmpty = signal(false);
  readonly fill = signal(false);
  readonly onRetry = vi.fn();
  readonly onOpen = vi.fn();
  readonly onPage = vi.fn();
}

function setup(): {
  fixture: ComponentFixture<DataTableHost>;
  host: DataTableHost;
  el: HTMLElement;
} {
  const fixture = TestBed.createComponent(DataTableHost);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

function byAria(root: ParentNode, label: string, selector = '*'): HTMLElement | undefined {
  return Array.from(root.querySelectorAll<HTMLElement>(selector)).find(
    (node) => node.getAttribute('aria-label') === label,
  );
}

function textOf(root: ParentNode, selector: string): string {
  return root.querySelector(selector)?.textContent?.trim() ?? '';
}

function rowFor(el: HTMLElement, text: string): HTMLElement {
  const cell = Array.from(el.querySelectorAll<HTMLElement>('td')).find(
    (td) => td.textContent?.trim() === text,
  );
  return cell?.closest('tr') as HTMLElement;
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// jsdom has no matchMedia; PrimeNG overlays query it on open and on destroy.
const defaultMatchMedia = ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener: () => {},
  removeEventListener: () => {},
  addListener: () => {},
  removeListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

beforeEach(() => {
  window.localStorage.clear();
  window.matchMedia = defaultMatchMedia;
  TestBed.configureTestingModule({ imports: [DataTableHost] });
});

afterEach(() => {
  window.matchMedia = defaultMatchMedia;
  document.querySelectorAll('.p-popover, .p-menu').forEach((node) => node.remove());
});

describe('DataTable', () => {
  it('shows skeleton rows after the loading delay', async () => {
    const { fixture, host, el } = setup();
    host.data.set([]);
    host.loading.set(true);
    host.pagination.set({ page: 1, pageSize: 20, total: 0 });
    fixture.detectChanges();

    expect(byAria(el, 'Cargando')).toBeUndefined();

    await wait(200);
    fixture.detectChanges();
    const region = byAria(el, 'Cargando') as HTMLElement;
    expect(region).toBeDefined();
    // Page size 20 is capped at 10 skeleton rows, plus the header row.
    expect(region.querySelectorAll('tr')).toHaveLength(11);
  });

  it('renders the default empty state or a custom empty slot', () => {
    const { fixture, host, el } = setup();
    host.data.set([]);
    fixture.detectChanges();
    expect(el.textContent).toContain('Sin resultados');

    host.useEmpty.set(true);
    fixture.detectChanges();
    expect(el.textContent).toContain('Aún no hay filas');
  });

  it('replaces the table with an error and calls retry', () => {
    const { fixture, host, el } = setup();
    host.error.set('Sin conexión con el servidor');
    fixture.detectChanges();

    const alert = el.querySelector('[role="alert"]') as HTMLElement;
    expect(alert.textContent).toContain('Sin conexión con el servidor');
    expect(el.querySelector('table')).toBeNull();

    (
      Array.from(el.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Reintentar'),
      ) as HTMLButtonElement
    ).click();
    expect(host.onRetry).toHaveBeenCalledTimes(1);
  });

  it('selects rows and shows BulkBar actions that can clear the selection', () => {
    const { fixture, host, el } = setup();
    const onDelete = vi.fn();
    host.selectable.set(true);
    host.selected.set(['1']);
    host.bulkActions.set([{ key: 'delete', label: 'Eliminar', danger: true, onClick: onDelete }]);
    fixture.detectChanges();

    const region = el.querySelector(
      '[role="region"][aria-label="Acciones masivas"]',
    ) as HTMLElement;
    expect(region.textContent).toContain('1 seleccionado');

    (
      Array.from(region.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Eliminar'),
      ) as HTMLButtonElement
    ).click();
    expect(onDelete).toHaveBeenCalledTimes(1);

    // Checkboxes: index 0 is the header "select all", index 2 is row 2.
    const checkboxes = el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
    checkboxes[2].click();
    fixture.detectChanges();
    expect(host.selected()).toEqual(['1', '2']);

    (
      Array.from(region.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Limpiar selección'),
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    expect(host.selected()).toEqual([]);
  });

  it('emits page and page size from the footer', () => {
    const { fixture, host, el } = setup();
    host.pagination.set({ page: 1, pageSize: 20, total: 100 });
    fixture.detectChanges();

    expect(el.textContent).toContain('100 elementos');
    (byAria(el, 'Página siguiente') as HTMLButtonElement).click();
    expect(host.onPage).toHaveBeenLastCalledWith({ page: 2, pageSize: 20 });
  });

  it('changes the page size from the footer selector (20, 50, 100)', async () => {
    const { fixture, host, el } = setup();
    host.pagination.set({ page: 1, pageSize: 20, total: 100 });
    fixture.detectChanges();

    const select = el.querySelector('p-select [role="combobox"], p-select') as HTMLElement;
    select.click();
    fixture.detectChanges();
    await wait(0);
    const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find((o) =>
      o.textContent?.includes('50 / página'),
    ) as HTMLElement;
    option.click();
    fixture.detectChanges();
    expect(host.onPage).toHaveBeenLastCalledWith({ page: 1, pageSize: 50 });
  });

  it('persists the density choice in corelog.density across remounts', () => {
    const first = setup();
    (byAria(first.el, 'Densidad') as HTMLElement).querySelectorAll('input')[1].click();
    first.fixture.detectChanges();
    expect(window.localStorage.getItem('corelog.density')).toBe('"compact"');
    first.fixture.destroy();

    const second = setup();
    const compact = Array.from(
      (byAria(second.el, 'Densidad') as HTMLElement).querySelectorAll('input'),
    ).find((input) => input.value === 'compact') as HTMLInputElement;
    expect(compact.checked).toBe(true);
  });

  it('persists hidden columns per tableKey across remounts', async () => {
    const first = setup();
    (
      Array.from(first.el.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Columnas'),
      ) as HTMLButtonElement
    ).click();
    first.fixture.detectChanges();
    await wait(0);
    const checkbox = Array.from(
      document.querySelectorAll<HTMLInputElement>(
        '.p-popover input[type="checkbox"], [role="group"][aria-label="Columnas visibles"] input',
      ),
    ).find((input) => input.closest('label')?.textContent?.trim() === 'Creado') as HTMLInputElement;
    checkbox.click();
    first.fixture.detectChanges();
    expect(first.el.querySelector('th:last-child')?.textContent).not.toContain('Creado');
    first.fixture.destroy();

    const second = setup();
    expect(
      Array.from(second.el.querySelectorAll('th')).map((th) => th.textContent?.trim()),
    ).not.toContain('Creado');
    expect(window.localStorage.getItem('corelog.table.test-table.columns')).toBe('["createdAt"]');
  });

  it('renders cards instead of the table below 768 px when mobileRender is set', () => {
    window.matchMedia = vi.fn((query: string) => ({
      matches: query === '(max-width: 767px)',
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;

    const { fixture, host, el } = setup();
    host.useMobile.set(true);
    fixture.detectChanges();

    expect(el.querySelector('table')).toBeNull();
    expect(el.textContent).toContain('Tarjeta Fila 2');
  });

  it('moves focus with arrow keys and opens a row with Enter', () => {
    const { fixture, host, el } = setup();
    host.rowOpenable.set(true);
    fixture.detectChanges();

    const firstRow = rowFor(el, 'Fila 1');
    const secondRow = rowFor(el, 'Fila 2');
    firstRow.focus();

    firstRow.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    expect(document.activeElement).toBe(secondRow);

    secondRow.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }));
    expect(document.activeElement).toBe(firstRow);

    firstRow.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    expect(host.onOpen).toHaveBeenCalledWith(rows[0]);

    (secondRow.querySelector('td') as HTMLElement).click();
    expect(host.onOpen).toHaveBeenLastCalledWith(rows[1]);
  });

  it('opens the row action menu and runs the chosen action', async () => {
    const onEdit = vi.fn();
    const { fixture, host, el } = setup();
    host.rowActions.set((row) => [{ key: 'edit', label: `Editar ${row.title}`, onClick: onEdit }]);
    fixture.detectChanges();

    el.querySelectorAll<HTMLButtonElement>('button[aria-label="Acciones de la fila"]')[0].click();
    fixture.detectChanges();
    await wait(0);
    fixture.detectChanges();

    const item = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(
      (node) => node.textContent?.includes('Editar Fila 1'),
    ) as HTMLElement;
    // PrimeNG handles the click on the content element inside the menuitem.
    (item.querySelector('.p-menu-item-content') as HTMLElement).click();
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it('shows no row action button for rows whose action list is empty', () => {
    const { fixture, host, el } = setup();
    host.rowActions.set((row) =>
      row.id === '2' ? [{ key: 'edit', label: 'Editar', onClick: vi.fn() }] : [],
    );
    fixture.detectChanges();
    expect(el.querySelectorAll('button[aria-label="Acciones de la fila"]')).toHaveLength(1);
  });

  it('omits the actions column entirely when no row has actions', () => {
    const { fixture, host, el } = setup();
    host.rowActions.set(() => []);
    fixture.detectChanges();
    expect(el.querySelector('button[aria-label="Acciones de la fila"]')).toBeNull();
    expect(el.querySelector('th.col-actions')).toBeNull();
  });

  it('waits for the loading delay before the skeleton is removed when data arrives', async () => {
    const { fixture, host, el } = setup();
    host.data.set([]);
    host.loading.set(true);
    fixture.detectChanges();
    await wait(200);
    fixture.detectChanges();
    expect(byAria(el, 'Cargando')).toBeDefined();

    host.data.set(rows);
    host.loading.set(false);
    fixture.detectChanges();
    expect(byAria(el, 'Cargando')).toBeUndefined();
    expect(el.textContent).toContain('Fila 3');
  });

  it('is not filled by default and fills its card with fill', () => {
    const { fixture, host, el } = setup();
    expect(el.querySelector('.root')?.getAttribute('data-fill')).toBe('false');

    host.fill.set(true);
    host.data.set([]);
    fixture.detectChanges();
    expect(el.querySelector('.root')?.getAttribute('data-fill')).toBe('true');
    expect(el.querySelector('.root')?.classList).toContain('fill');
    expect(el.querySelector('.area')).not.toBeNull();
  });

  it('keeps the empty state at the top of a filled table', () => {
    const { fixture, host, el } = setup();
    host.fill.set(true);
    host.data.set([]);
    fixture.detectChanges();
    expect(el.querySelector('.area .empty')).not.toBeNull();
    expect(el.textContent).toContain('Sin resultados');
  });

  it('gives truncated string cells a title with the full value', () => {
    const { fixture, host, el } = setup();
    const long = 'Un título muy largo que no cabe en la celda';
    host.columns.set([{ key: 'title', title: 'Título', field: 'title', ellipsis: true }]);
    host.data.set([{ id: '9', title: long, createdAt: '2026-11-09' }]);
    fixture.detectChanges();
    expect(rowFor(el, long).querySelector('td')?.getAttribute('title')).toBe(long);
  });

  it('sorts the visible rows when a sortable column header is activated', () => {
    const { fixture, host, el } = setup();
    host.columns.set([{ key: 'title', title: 'Título', field: 'title', sortable: true }]);
    host.data.set([
      { id: 'b', title: 'Beta', createdAt: '' },
      { id: 'a', title: 'Alfa', createdAt: '' },
    ]);
    fixture.detectChanges();

    const sortButton = el.querySelector('th button') as HTMLButtonElement;
    sortButton.click();
    fixture.detectChanges();
    expect(el.querySelector('th')?.getAttribute('aria-sort')).toBe('ascending');
    expect(textOf(el, 'tbody tr td')).toBe('Alfa');

    sortButton.click();
    fixture.detectChanges();
    expect(el.querySelector('th')?.getAttribute('aria-sort')).toBe('descending');
    expect(textOf(el, 'tbody tr td')).toBe('Beta');
  });
});
