import { NgTemplateOutlet } from '@angular/common';
import { booleanAttribute, ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * Grid of equal-height cards. Items stretch to the tallest card in their row and the last row keeps
 * its cards on the left (no orphan gaps).
 *
 * With fill, the grid sits in a surface panel that grows to the bottom of a filled page, so a short
 * list leaves no bare band under its cards.
 */
@Component({
  selector: 'app-card-grid',
  imports: [NgTemplateOutlet],
  templateUrl: './card-grid.html',
  styleUrl: './card-grid.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CardGrid {
  /** Minimum card width in px; columns fill the row with repeat(auto-fill, minmax(min, 1fr)). Default 280. */
  readonly minCardWidth = input(280);
  /** Accessible name of the grid group; set it when the cards have no heading of their own. */
  readonly ariaLabel = input<string>();
  /** Frames the grid in a surface panel that grows to the bottom of a filled page. Default false. */
  readonly fill = input(false, { transform: booleanAttribute });
}
