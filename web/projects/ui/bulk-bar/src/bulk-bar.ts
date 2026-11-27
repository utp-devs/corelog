import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { Button } from 'primeng/button';

/** Bulk action shown in the bar. `danger` renders the action with the danger severity. */
export interface BulkAction {
  readonly key: string;
  readonly label: string;
  readonly icon?: string;
  readonly danger?: boolean;
  readonly onClick: () => void;
}

/** Bulk actions bar: appears above a table when rows are selected. */
@Component({
  selector: 'app-bulk-bar',
  imports: [Button],
  templateUrl: './bulk-bar.html',
  styleUrl: './bulk-bar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BulkBar {
  /** Number of selected rows. The bar is only shown when this is greater than zero. */
  readonly count = input(0);
  readonly actions = input<BulkAction[]>([]);
  /** Clears the selection ("Limpiar selección" button). */
  readonly clear = output<void>();

  protected readonly countText = computed(() =>
    this.count() === 1 ? '1 seleccionado' : `${this.count()} seleccionados`,
  );
}
