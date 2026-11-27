import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export type ProgressTone = 'brand' | 'success' | 'warning' | 'danger';

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.min(100, Math.max(0, Math.round(value)));
}

/** Accessible progress bar. The visible percentage and the accessible value share one clamped number. */
@Component({
  selector: 'app-progress-bar',
  templateUrl: './progress-bar.html',
  styleUrl: './progress-bar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProgressBar {
  /** Percentage 0-100; values outside the range are clamped. */
  readonly value = input.required<number>();
  /** Visible text and accessible name, for example "Completado". */
  readonly label = input.required<string>();
  readonly tone = input<ProgressTone>('brand');
  /** Test hook, rendered as data-testid on the wrapper. */
  readonly testId = input<string>('', { alias: 'data-testid' });

  protected readonly percent = computed(() => clampPercent(this.value()));
}
