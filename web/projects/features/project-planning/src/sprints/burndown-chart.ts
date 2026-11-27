import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { BurndownPoint } from '@corelog/core';
import { ChartCard, ChartCardSeries, ChartMarker, ChartTable } from '@corelog/ui/chart-card';

const DATE_FORMAT = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short' });
const ISO_DAY = /^(\d{4})-(\d{2})-(\d{2})/;

/** Formats an ISO day ("2026-11-02") as a short Spanish date ("2 nov"); other text is returned unchanged. */
export function formatBurndownDate(value: string): string {
  const match = ISO_DAY.exec(value);
  if (!match) return value;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return DATE_FORMAT.format(date).replace('.', '');
}

function todayIso(): string {
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/**
 * Burndown of a sprint: remaining work (solid) against the ideal line (dashed), with a marker on
 * today, drawn with the shared chart card, which also offers the same data as a table.
 */
@Component({
  selector: 'app-burndown-chart',
  imports: [ChartCard],
  templateUrl: './burndown-chart.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BurndownChart {
  readonly points = input.required<readonly BurndownPoint[]>();

  protected readonly labels = computed(() =>
    this.points().map((point) => formatBurndownDate(point.date)),
  );
  protected readonly series = computed<ChartCardSeries[]>(() => [
    { label: 'Ideal', data: this.points().map((point) => point.ideal), dashed: true },
    { label: 'Restante', data: this.points().map((point) => point.remaining) },
  ]);
  protected readonly table = computed<ChartTable>(() => ({
    headers: ['Fecha', 'Ideal', 'Restante'],
    rows: this.points().map((point, index) => [this.labels()[index], point.ideal, point.remaining]),
  }));
  /** Today's position on the date axis; absent when today is outside the sprint. */
  protected readonly marker = computed<ChartMarker | undefined>(() => {
    const today = todayIso();
    const index = this.points().findIndex((point) => point.date.startsWith(today));
    return index >= 0 ? { index, label: 'Hoy' } : undefined;
  });
}
