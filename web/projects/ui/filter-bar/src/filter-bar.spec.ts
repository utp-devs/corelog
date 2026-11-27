import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { FilterBar, FilterDefinition } from './filter-bar';

const statusFilter: FilterDefinition = {
  key: 'status',
  label: 'Estado',
  options: [{ value: 'open', label: 'Abierto' }],
  onChange: () => {},
};

@Component({
  imports: [FilterBar],
  template: `
    <ng-template #actions><button type="button">Guardar filtro</button></ng-template>
    <app-filter-bar
      [(searchValue)]="search"
      [filters]="filters()"
      [actions]="withActions() ? actions : null"
      (clear)="onClear()"
    />
  `,
})
class FilterBarHost {
  readonly search = signal('');
  readonly filters = signal<FilterDefinition[]>([]);
  readonly withActions = signal(false);
  readonly onClear = vi.fn();
}

function create(): ComponentFixture<FilterBarHost> {
  TestBed.configureTestingModule({ imports: [FilterBarHost] });
  const fixture = TestBed.createComponent(FilterBarHost);
  fixture.detectChanges();
  return fixture;
}

function buttonByText(el: HTMLElement, text: string): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim() === text);
}

describe('FilterBar', () => {
  it('renders the search field and each filter selector', () => {
    const fixture = create();
    fixture.componentInstance.filters.set([statusFilter]);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;

    expect(el.querySelector('input[aria-label="Buscar"]')).not.toBeNull();
    expect(
      el.querySelector('[role="combobox"][aria-label="Estado"], p-select[ariaLabel="Estado"]'),
    ).not.toBeNull();
  });

  it('shows the clear action only when a filter is active', () => {
    const fixture = create();
    const el = fixture.nativeElement as HTMLElement;
    expect(buttonByText(el, 'Limpiar filtros')).toBeUndefined();

    fixture.componentInstance.search.set('login');
    fixture.detectChanges();
    buttonByText(el, 'Limpiar filtros')?.click();
    expect(fixture.componentInstance.onClear).toHaveBeenCalledTimes(1);
  });

  it('renders the extra actions slot', () => {
    const fixture = create();
    fixture.componentInstance.withActions.set(true);
    fixture.detectChanges();
    expect(buttonByText(fixture.nativeElement as HTMLElement, 'Guardar filtro')).toBeDefined();
  });

  it('shows one removable chip per active filter and for the search text', () => {
    const fixture = create();
    const onChange = vi.fn();
    fixture.componentInstance.filters.set([{ ...statusFilter, value: 'open', onChange }]);
    fixture.componentInstance.search.set('login');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;

    const chips = Array.from(el.querySelectorAll('.filter-bar__chip-text')).map(
      (c) => c.textContent,
    );
    expect(chips).toEqual(['Búsqueda: login', 'Estado: Abierto']);

    (
      el.querySelector('button[aria-label="Quitar filtro Estado: Abierto"]') as HTMLButtonElement
    ).click();
    expect(onChange).toHaveBeenCalledWith(undefined);

    (
      el.querySelector('button[aria-label="Quitar filtro Búsqueda: login"]') as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    expect(fixture.componentInstance.search()).toBe('');
  });

  it('shows no chips when nothing is active', () => {
    const fixture = create();
    expect((fixture.nativeElement as HTMLElement).querySelector('.filter-bar__chips')).toBeNull();
  });
});
