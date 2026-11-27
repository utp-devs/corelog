import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { RouterLink } from '@angular/router';
import { initialsOf, paletteIndex } from '@corelog/ui/avatar';

/** Team with a color tile, initials, name and member count. */
@Component({
  selector: 'app-team-chip',
  imports: [NgTemplateOutlet, RouterLink],
  templateUrl: './team-chip.html',
  styleUrl: './team-chip.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamChip {
  /** Stable team identifier: decides the color. */
  readonly id = input.required<string>();
  readonly name = input.required<string>();
  readonly memberCount = input<number>();
  /** When present, the chip is a link to the team. In-app paths (leading slash) use the router. */
  readonly href = input<string>();
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  protected readonly initials = computed(() => initialsOf(this.name()));
  /** In-app paths navigate through the router; absolute URLs stay plain links. */
  protected readonly isAppPath = computed(() => {
    const href = this.href() ?? '';
    return href.startsWith('/') && !href.startsWith('//');
  });

  protected readonly palette = computed(() => paletteIndex(this.id()));
  protected readonly memberText = computed(() => {
    const count = this.memberCount();
    if (count === undefined) {
      return null;
    }
    return count === 1 ? '1 miembro' : `${count} miembros`;
  });
}
