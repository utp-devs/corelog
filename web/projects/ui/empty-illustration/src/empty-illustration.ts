import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export type EmptyIllustrationKind = 'projects' | 'issues' | 'search' | 'notes' | 'generic';

/** Photograph for an empty, error or maintenance state: the file name under /images/states without extension. */
export type StateImage =
  | 'empty-projects'
  | 'empty-issues'
  | 'empty-search'
  | 'empty-notifications'
  | 'empty-teams'
  | 'error-403'
  | 'error-404'
  | 'error-500'
  | 'maintenance';

/**
 * Picture for empty states. It is decorative: the state text sits next to it. With `image` it draws a state
 * photograph in a rounded frame with its own dark background (the same in both themes); without it, a small
 * line drawing chosen by `kind`. Colors come from design tokens.
 */
@Component({
  selector: 'app-empty-illustration',
  templateUrl: './empty-illustration.html',
  styleUrl: './empty-illustration.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EmptyIllustration {
  readonly kind = input<EmptyIllustrationKind>('generic');
  /** State photograph; replaces the line drawing. */
  readonly image = input<StateImage>();
  readonly testId = input<string>(undefined, { alias: 'data-testid' });

  protected readonly imageSrc = computed(() => {
    const name = this.image();
    return name ? `/images/states/${name}.webp` : null;
  });
}
