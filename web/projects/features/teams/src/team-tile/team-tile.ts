import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { isHexColor } from '@corelog/ui/color-picker';
import { initialsOf, paletteIndex } from '@corelog/ui/avatar';

export type TeamTileSize = 'md' | 'lg';

/**
 * Square tile with the team initials in the team color. Decorative: the team name is rendered next to it.
 * A hex color chosen for the team wins; otherwise the color is derived from the team id.
 */
@Component({
  selector: 'app-team-tile',
  templateUrl: './team-tile.html',
  styleUrl: './team-tile.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamTile {
  /** Stable team identifier: decides the palette color when no custom color is set. */
  readonly id = input.required<string>();
  readonly name = input.required<string>();
  /** Hex color chosen for the team, or null to use the color derived from the id. */
  readonly color = input.required<string | null>();
  readonly size = input<TeamTileSize>('md');

  protected readonly initials = computed(() => initialsOf(this.name()));
  protected readonly customColor = computed(() => {
    const value = this.color();
    return value !== null && isHexColor(value) ? value : null;
  });
  protected readonly palette = computed(() => paletteIndex(this.id()));
}
