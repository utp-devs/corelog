import { ChangeDetectionStrategy, Component, input } from '@angular/core';

export type EmptyIllustrationKind = 'projects' | 'issues' | 'search' | 'notes' | 'generic';

/**
 * Small drawing for empty states. It is decorative: the state text sits next to it.
 * Colors come from design tokens through the accent and muted classes.
 */
@Component({
  selector: 'app-empty-illustration',
  templateUrl: './empty-illustration.html',
  styleUrl: './empty-illustration.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EmptyIllustration {
  readonly kind = input.required<EmptyIllustrationKind>();
  readonly testId = input<string>(undefined, { alias: 'data-testid' });
}
