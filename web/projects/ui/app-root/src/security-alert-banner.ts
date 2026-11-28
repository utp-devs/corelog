import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { NotificationStream, SecurityAlertActions, type NotificationAction } from '@corelog/core';

/**
 * Prominent alert for a login from a new device. It stays until the user acts (or the stream
 * drops it after a minute). "No fui yo" closes the new session and offers changing the password;
 * "Ver mis sesiones" opens the sessions page.
 */
@Component({
  selector: 'app-security-alert-banner',
  templateUrl: './security-alert-banner.html',
  styleUrl: './security-alert-banner.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SecurityAlertBanner {
  private readonly stream = inject(NotificationStream);
  private readonly actions = inject(SecurityAlertActions);

  protected readonly alert = this.stream.securityAlert;
  protected readonly busy = signal(false);
  /** True once the unknown session was closed: the banner then offers the password change. */
  protected readonly closed = signal(false);
  protected readonly buttons = computed(() =>
    this.actions.actionsOf({ actions: this.alert()?.actions }),
  );

  constructor() {
    // A new alert starts from its own initial state.
    effect(() => {
      this.alert();
      this.busy.set(false);
      this.closed.set(false);
    });
  }

  protected isNotMe(action: NotificationAction): boolean {
    return this.actions.isNotMe(action);
  }

  protected async run(action: NotificationAction): Promise<void> {
    if (this.busy()) return;
    if (!this.actions.isNotMe(action)) {
      this.stream.dismissAlert();
      this.actions.open(action.link);
      return;
    }
    this.busy.set(true);
    const ok = await this.actions.notMe(this.alert()?.sessionId);
    this.busy.set(false);
    this.closed.set(ok);
  }

  protected changePassword(): void {
    this.stream.dismissAlert();
    this.actions.open();
  }

  protected dismiss(): void {
    this.stream.dismissAlert();
  }
}
