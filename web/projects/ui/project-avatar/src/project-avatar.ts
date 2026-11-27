import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { paletteIndex } from '@corelog/ui/avatar';

export type ProjectAvatarSize = 24 | 32 | 40;

/** Color square with an icon or the initials of the project key. */
@Component({
  selector: 'app-project-avatar',
  templateUrl: './project-avatar.html',
  styleUrl: './project-avatar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectAvatar {
  /** Project name: used for the accessible name. */
  readonly name = input.required<string>();
  /** Project key (for example CL): decides the color and, without an icon, the initials. */
  readonly projectKey = input.required<string>();
  /** Optional PrimeIcons class name, for example "pi pi-star". Replaces the initials. */
  readonly icon = input<string>();
  readonly size = input<ProjectAvatarSize>(32);
  readonly dataTestId = input<string | undefined>(undefined, { alias: 'data-testid' });

  protected readonly initials = computed(() => this.projectKey().trim().toUpperCase().slice(0, 2));
  protected readonly palette = computed(() => paletteIndex(this.projectKey()));
  protected readonly sizeClass = computed(() => `size-${this.size()}`);
}
