import { ChangeDetectionStrategy, Component, input, output, signal } from '@angular/core';
import type { CalendarFeedSecret, CalendarFeedState } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';

/**
 * Subscription to the personal calendar from another app (an ICS feed). The address is shown a single time,
 * right after it is created; creating a new one replaces the old address, and revoking stops it.
 */
@Component({
  selector: 'app-feed-dialog',
  imports: [Button, Dialog],
  templateUrl: './feed-dialog.html',
  styleUrl: './feed-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FeedDialog {
  readonly open = input(false);
  readonly state = input<CalendarFeedState | null>(null);
  /** The address just created; it is not available again later. */
  readonly secret = input<CalendarFeedSecret | null>(null);
  readonly busy = input(false);
  readonly error = input<string | null>(null);

  readonly generate = output<void>();
  readonly revoke = output<void>();
  readonly closeRequested = output<void>();

  protected readonly copied = signal(false);

  protected async copy(url: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(url);
      this.copied.set(true);
      setTimeout(() => this.copied.set(false), 2500);
    } catch {
      this.copied.set(false);
    }
  }
}
