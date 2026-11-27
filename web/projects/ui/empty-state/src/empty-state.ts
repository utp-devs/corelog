import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { EmptyIllustration, type StateImage } from '@corelog/ui/empty-illustration';

/**
 * Empty state of a list, table or panel: icon, title, description and an optional action.
 * The icon is projected into the `[icon]` slot and the action into the `[action]` slot. With `image` a state
 * photograph in a rounded frame replaces the icon.
 */
@Component({
  selector: 'app-empty-state',
  imports: [EmptyIllustration],
  templateUrl: './empty-state.html',
  styleUrl: './empty-state.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EmptyState {
  readonly title = input.required<string>();
  readonly description = input<string>();
  /** State photograph shown above the text instead of the icon. */
  readonly image = input<StateImage>();
}
