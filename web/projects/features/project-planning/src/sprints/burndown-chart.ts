import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { BurndownPoint } from '@corelog/core';
import { ChartCard, ChartCardSeries, ChartTable } from '@corelog/ui/chart-card';

/**
 * Burndown of a sprint: remaining work against the ideal line, drawn with the shared chart card, which
 * also offers the same data as a table.
 */
@Component({
  selector: 'app-burndown-chart',
  imports: [ChartCard],
  templateUrl: './burndown-chart.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BurndownChart {
  readonly points = input.required<readonly BurndownPoint[]>();

  protected readonly labels = computed(() => this.points().map((point) => point.date));
  protected readonly series = computed<ChartCardSeries[]>(() => [
    { label: 'Ideal', data: this.points().map((point) => point.ideal) },
    { label: 'Restante', data: this.points().map((point) => point.remaining) },
  ]);
  protected readonly table = computed<ChartTable>(() => ({
    headers: ['Fecha', 'Ideal', 'Restante'],
    rows: this.points().map((point) => [point.date, point.ideal, point.remaining]),
  }));
}
