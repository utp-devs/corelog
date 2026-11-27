import {
  ChangeDetectionStrategy,
  Component,
  TemplateRef,
  computed,
  input,
  model,
  output,
} from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { SearchInput } from '@corelog/ui/search-input';
import { SelectField } from '@corelog/ui/select-field';

export interface FilterOption {
  readonly value: string;
  readonly label: string;
}

/** One selector of the bar. `value` is undefined when nothing is selected. */
export interface FilterDefinition {
  readonly key: string;
  readonly label: string;
  readonly options: FilterOption[];
  readonly value?: string;
  readonly onChange: (value?: string) => void;
}

interface FilterChip {
  readonly key: string;
  readonly text: string;
  /** Filter to reset; undefined for the search chip. */
  readonly filter?: FilterDefinition;
}

/**
 * Filter bar: debounced search, one selector per field, a "Limpiar filtros" action and one removable chip
 * per active filter (so the user sees what is applied without reading every select).
 */
@Component({
  selector: 'app-filter-bar',
  imports: [NgTemplateOutlet, FormsModule, SearchInput, SelectField],
  templateUrl: './filter-bar.html',
  styleUrl: './filter-bar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FilterBar {
  /** Search text (two-way bound). */
  readonly searchValue = model<string>('');
  readonly searchPlaceholder = input('Buscar');
  readonly filters = input<FilterDefinition[]>([]);
  /** Extra actions on the right, for example "Guardar filtro". */
  readonly actions = input<TemplateRef<void> | null>(null);
  /** Clears the search and the filters. The button only appears when something is active. */
  readonly clear = output<void>();

  protected readonly hasActive = computed(
    () =>
      this.searchValue() !== '' ||
      this.filters().some((filter) => filter.value !== undefined && filter.value !== ''),
  );

  protected readonly chips = computed<FilterChip[]>(() => {
    const chips: FilterChip[] = [];
    const search = this.searchValue().trim();
    if (search !== '') {
      chips.push({ key: 'search', text: `Búsqueda: ${search}` });
    }
    for (const filter of this.filters()) {
      if (filter.value === undefined || filter.value === '') {
        continue;
      }
      const label = filter.options.find((option) => option.value === filter.value)?.label;
      chips.push({ key: filter.key, text: `${filter.label}: ${label ?? filter.value}`, filter });
    }
    return chips;
  });

  protected removeChip(chip: FilterChip): void {
    if (chip.filter) {
      chip.filter.onChange(undefined);
    } else {
      this.searchValue.set('');
    }
  }

  protected onFilterChange(filter: FilterDefinition, value: string | null): void {
    filter.onChange(value ?? undefined);
  }
}
