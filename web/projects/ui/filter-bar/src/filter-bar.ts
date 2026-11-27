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
import { Button } from 'primeng/button';
import { Select } from 'primeng/select';

import { SearchInput } from '@corelog/ui/search-input';

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

/** Filter bar: debounced search, one selector per field and a clear action. */
@Component({
  selector: 'app-filter-bar',
  imports: [NgTemplateOutlet, FormsModule, Button, Select, SearchInput],
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

  protected onFilterChange(filter: FilterDefinition, value: string | null): void {
    filter.onChange(value ?? undefined);
  }
}
