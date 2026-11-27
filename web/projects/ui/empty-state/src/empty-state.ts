import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * Empty state of a list, table or panel: icon, title, description and an optional action.
 * The icon is projected into the `[icon]` slot and the action into the `[action]` slot.
 */
@Component({
  selector: 'app-empty-state',
  templateUrl: './empty-state.html',
  styleUrl: './empty-state.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EmptyState {
  readonly title = input.required<string>();
  readonly description = input<string>();
}
