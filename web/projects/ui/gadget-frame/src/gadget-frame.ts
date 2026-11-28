import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { AppTooltip } from '@corelog/ui/tooltip';

export type GadgetFrameState = 'loading' | 'error' | 'ready';

let nextFrameId = 0;

/**
 * Card of a dashboard gadget: title, optional update time, a refresh button and an actions slot in the
 * header, and a body that shows a skeleton while loading, an error with a retry button, or the content.
 * Extra header controls (edit mode actions) go in the `[gadgetFrameActions]` slot.
 */
@Component({
  selector: 'app-gadget-frame',
  imports: [AppTooltip, LoadingSkeleton],
  templateUrl: './gadget-frame.html',
  styleUrl: './gadget-frame.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GadgetFrame {
  readonly title = input.required<string>();
  readonly state = input<GadgetFrameState>('ready');
  readonly errorMessage = input('No se pudo cargar este gadget.');
  /** Text under the title, for example the time of the last update. */
  readonly subtitle = input<string>();
  /** Shows the refresh button. */
  readonly refreshable = input(true);
  /** The data is being reloaded while the old content stays visible. */
  readonly refreshing = input(false);
  /** Edit mode: the frame shows a dashed outline and a grab affordance. */
  readonly editing = input(false);
  /** Lets the body fill the frame (charts, text). */
  readonly flush = input(false);

  readonly refresh = output<void>();

  protected readonly titleId = `cl-gadget-title-${nextFrameId++}`;
  protected readonly showRefresh = computed(() => this.refreshable() && !this.editing());
}
