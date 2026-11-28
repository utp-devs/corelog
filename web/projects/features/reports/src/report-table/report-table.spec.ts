import { ComponentFixture, TestBed } from '@angular/core/testing';
import type { ReportTable as ReportTableData } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { PAGE_ROWS, ReportTable } from './report-table';

const TABLE: ReportTableData = {
  columns: [
    { key: 'name', label: 'Persona', type: 'text' },
    { key: 'open', label: 'Abiertas', type: 'number' },
    { key: 'due', label: 'Vence', type: 'date' },
    { key: 'ratio', label: 'Cumplimiento', type: 'percent' },
  ],
  rows: [
    ['Bea', 10, '2026-11-05', 50],
    ['Ana', 2, '2026-11-01', null],
    ['Cris', 33, null, 12.5],
  ],
};

function create(table: ReportTableData = TABLE): ComponentFixture<ReportTable> {
  const fixture = TestBed.createComponent(ReportTable);
  fixture.componentRef.setInput('table', table);
  fixture.componentRef.setInput('title', 'Carga de trabajo');
  fixture.detectChanges();
  return fixture;
}

function column(fixture: ComponentFixture<ReportTable>, index: number): string[] {
  return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('tbody tr')).map(
    (row) => row.querySelectorAll('td')[index].textContent?.trim() ?? '',
  );
}

describe('ReportTable', () => {
  it('formats the cells by column type', () => {
    const fixture = create();
    expect(column(fixture, 1)).toEqual(['10', '2', '33']);
    expect(column(fixture, 2)).toEqual(['05/11/2026', '01/11/2026', '–']);
    expect(column(fixture, 3)).toEqual(['50 %', '–', '12,5 %']);
  });

  it('has a named region, a caption and column headers', () => {
    const el = create().nativeElement as HTMLElement;
    expect(el.querySelector('[role="region"]')?.getAttribute('aria-labelledby')).toBe(
      el.querySelector('caption')?.id,
    );
    expect(el.querySelector('caption')?.textContent).toContain('Carga de trabajo');
    expect(el.querySelectorAll('th[scope="col"]')).toHaveLength(4);
  });

  it('sorts ascending, descending and back to the original order', () => {
    const fixture = create();
    const el = fixture.nativeElement as HTMLElement;
    const sortButton = el.querySelector<HTMLButtonElement>('[data-testid="report-sort-name"]');
    const header = el.querySelector('th');

    expect(header?.getAttribute('aria-sort')).toBe('none');
    sortButton?.click();
    fixture.detectChanges();
    expect(column(fixture, 1)).toEqual(['2', '10', '33']);
    expect(header?.getAttribute('aria-sort')).toBe('ascending');

    sortButton?.click();
    fixture.detectChanges();
    expect(column(fixture, 1)).toEqual(['33', '10', '2']);
    expect(header?.getAttribute('aria-sort')).toBe('descending');

    sortButton?.click();
    fixture.detectChanges();
    expect(column(fixture, 1)).toEqual(['10', '2', '33']);
    expect(header?.getAttribute('aria-sort')).toBe('none');
  });

  it('limits the rows and shows more on demand', () => {
    const rows = Array.from({ length: PAGE_ROWS + 20 }, (_, index) => [
      `P${index}`,
      index,
      null,
      null,
    ]);
    const fixture = create({ ...TABLE, rows });
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('tbody tr')).toHaveLength(PAGE_ROWS);
    const more = el.querySelector<HTMLButtonElement>('[data-testid="report-show-more"]');
    expect(more?.textContent).toContain('20 filas');
    more?.click();
    fixture.detectChanges();
    expect(el.querySelectorAll('tbody tr')).toHaveLength(PAGE_ROWS + 20);
    expect(el.querySelector('[data-testid="report-show-more"]')).toBeNull();
  });
});
