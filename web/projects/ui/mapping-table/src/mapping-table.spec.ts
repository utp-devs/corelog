import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { assignField, fieldByColumn, missingRequired, pruneMapping } from './mapping';
import { MappingTable } from './mapping-table';

describe('mapping helpers', () => {
  it('inverts the mapping by column', () => {
    expect(fieldByColumn({ summary: 'Titulo', labels: 'Tags' })).toEqual({
      Titulo: 'summary',
      Tags: 'labels',
    });
  });

  it('keeps one column per field and one field per column', () => {
    let mapping = assignField({}, 'A', 'summary');
    mapping = assignField(mapping, 'B', 'summary');
    expect(mapping).toEqual({ summary: 'B' });
    mapping = assignField(mapping, 'B', 'labels');
    expect(mapping).toEqual({ labels: 'B' });
    expect(assignField(mapping, 'B', '')).toEqual({});
  });

  it('reports required fields without a column', () => {
    expect(missingRequired({}).map((f) => f.key)).toEqual(['summary']);
    expect(missingRequired({ summary: 'A' })).toEqual([]);
  });

  it('drops mapping entries whose column is not in the file', () => {
    expect(pruneMapping({ summary: 'A', labels: 'Z' }, ['A'])).toEqual({ summary: 'A' });
  });
});

@Component({
  imports: [MappingTable],
  template: `<app-mapping-table [columns]="columns" [sampleRows]="rows" [(mapping)]="mapping" />`,
})
class Host {
  columns = ['Titulo', 'Tags'];
  rows = [
    ['Uno', 'a'],
    ['Dos', 'b'],
  ];
  readonly mapping = signal<Record<string, string>>({});
}

describe('MappingTable', () => {
  function setup() {
    TestBed.configureTestingModule({ imports: [Host] });
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, host: fixture.componentInstance };
  }

  it('flags the missing summary and clears the flag once mapped', () => {
    const { fixture, el, host } = setup();
    expect(el.querySelector('[data-testid="mapping-missing"]')?.textContent).toContain('Resumen');
    const select = el.querySelector<HTMLSelectElement>('[data-testid="mapping-select-0"]')!;
    select.value = 'summary';
    select.dispatchEvent(new Event('change'));
    fixture.detectChanges();
    expect(host.mapping()).toEqual({ summary: 'Titulo' });
    expect(el.querySelector('[data-testid="mapping-missing"]')).toBeNull();
  });

  it('shows sample values per column and disables a field used by another column', () => {
    const { fixture, el, host } = setup();
    host.mapping.set({ summary: 'Titulo' });
    fixture.changeDetectorRef.detectChanges();
    fixture.detectChanges();
    expect(el.textContent).toContain('Uno');
    const options = el.querySelectorAll<HTMLOptionElement>(
      '[data-testid="mapping-select-1"] option',
    );
    const summary = Array.from(options).find((o) => o.value === 'summary');
    expect(summary?.disabled).toBe(true);
  });
});
