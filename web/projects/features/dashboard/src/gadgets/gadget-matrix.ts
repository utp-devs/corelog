import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import type { GadgetData, MatrixGadgetData } from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';

import { groupLabel } from './gadget-format';

/** Two-dimensional statistics as a table: one field in the rows, another in the columns. */
@Component({
  selector: 'app-gadget-matrix',
  imports: [EmptyState],
  templateUrl: './gadget-matrix.html',
  styleUrl: './gadget-matrix.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GadgetMatrix {
  readonly content = input.required<GadgetData>();

  protected readonly data = computed<MatrixGadgetData | null>(() => {
    const content = this.content();
    return content.type === 'two_dimensional' ? content.data : null;
  });

  private readonly counts = computed(() => {
    const map = new Map<string, number>();
    for (const cell of this.data()?.cells ?? []) map.set(`${cell.x}\u0000${cell.y}`, cell.count);
    return map;
  });

  protected count(x: string, y: string): number {
    return this.counts().get(`${x}\u0000${y}`) ?? 0;
  }

  protected label(value: string): string {
    return groupLabel(value);
  }
}
