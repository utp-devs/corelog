import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { WipState } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';

/** Spanish text of each WIP state, for the tooltip and the screen reader. */
export const WIP_LABELS: Readonly<Record<WipState, string>> = {
  ok: 'Dentro del límite',
  below_min: 'Por debajo del mínimo',
  at_max: 'Límite alcanzado',
  over_max: 'Límite superado',
};

/**
 * Header of a board column: name, number of cards, the estimate and the work-in-progress indicator.
 * The WIP indicator is advisory: it changes color and text but never blocks anything. Colors come from
 * tokens and the state is also written, so it does not rely on color alone.
 */
@Component({
  selector: 'app-board-column-header',
  imports: [AppTooltip],
  templateUrl: './board-column-header.html',
  styleUrl: './board-column-header.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BoardColumnHeader {
  readonly name = input.required<string>();
  /** Cards shown in the column. */
  readonly count = input.required<number>();
  /** Cards of the column before the quick filters; the limits apply to it. */
  readonly total = input<number>();
  /** Sum under the board estimation, already formatted ("8 pts"); empty hides it. */
  readonly estimate = input('');
  readonly wipState = input<WipState>('ok');
  readonly min = input<number | null>(null);
  readonly max = input<number | null>(null);
  /** Color name of the column accent bar (gray, red, ... ); empty for the default. */
  readonly color = input('');

  protected readonly hasLimit = computed(() => this.max() !== null || this.min() !== null);
  protected readonly stateLabel = computed(() => WIP_LABELS[this.wipState()]);
  protected readonly limitText = computed(() => {
    const parts: string[] = [];
    if (this.min() !== null) parts.push(`mínimo ${this.min()}`);
    if (this.max() !== null) parts.push(`máximo ${this.max()}`);
    return parts.join(', ');
  });
  protected readonly countText = computed(() => {
    const max = this.max();
    return max !== null ? `${this.total() ?? this.count()}/${max}` : String(this.count());
  });
  protected readonly tooltip = computed(() =>
    this.hasLimit()
      ? `${this.stateLabel()} (${this.limitText()})`
      : 'Sin límite de trabajo en curso',
  );
  protected readonly colorClass = computed(() =>
    this.color() ? `board-column-header--${this.color()}` : '',
  );
}
