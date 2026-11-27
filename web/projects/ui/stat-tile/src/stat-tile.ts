import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/** KPI compact tile for dashboards and reports: label, highlighted value and optional context line. */
@Component({
  selector: 'app-stat-tile',
  templateUrl: './stat-tile.html',
  styleUrl: './stat-tile.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class StatTile {
  readonly label = input.required<string>();
  /** Value shown in the tile. */
  readonly value = input.required<string | number>();
  /** Secondary text under the value, for example a variation or a unit. */
  readonly hint = input<string>();
}
