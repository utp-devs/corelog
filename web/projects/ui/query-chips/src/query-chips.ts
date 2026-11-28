import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import type { QueryChip } from '@corelog/core';

/** Removable chips of the active basic filters of the issue navigator. */
@Component({
  selector: 'app-query-chips',
  templateUrl: './query-chips.html',
  styleUrl: './query-chips.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class QueryChips {
  readonly chips = input<readonly QueryChip[]>([]);
  readonly remove = output<QueryChip>();
  readonly clear = output<void>();
}
