import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { NgTemplateOutlet } from '@angular/common';
import { initialsOf, paletteIndex } from '@corelog/ui/avatar';

/** Team with a color tile, initials, name and member count. */
@Component({
  selector: 'app-team-chip',
  imports: [NgTemplateOutlet],
  templateUrl: './team-chip.html',
  styleUrl: './team-chip.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamChip {
  /** Stable team identifier: decides the color. */
  readonly id = input.required<string>();
  readonly name = input.required<string>();
  readonly memberCount = input<number>();
  /** When present, the chip is a link to the team. */
  readonly href = input<string>();
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  protected readonly initials = computed(() => initialsOf(this.name()));
  protected readonly palette = computed(() => paletteIndex(this.id()));
  protected readonly memberText = computed(() => {
    const count = this.memberCount();
    if (count === undefined) {
      return null;
    }
    return count === 1 ? '1 miembro' : `${count} miembros`;
  });
}
