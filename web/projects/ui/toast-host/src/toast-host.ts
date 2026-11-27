import { ChangeDetectionStrategy, Component, ViewEncapsulation, inject } from '@angular/core';
import { CL_Z_INDEX, NotificationService } from '@corelog/core';
import type { ToastCloseEvent } from 'primeng/types/toast';
import { Toast } from 'primeng/toast';

/**
 * Outlet of the toasts sent through NotificationService: top right, below the header, at the token
 * width, above dialogs. Hovering a toast pauses its auto-close. The styles are global on purpose: PrimeNG
 * renders the toast container itself, so its position can only be set from a class passed to it.
 */
@Component({
  selector: 'app-toast-host',
  imports: [Toast],
  templateUrl: './toast-host.html',
  styleUrl: './toast-host.scss',
  encapsulation: ViewEncapsulation.None,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ToastHost {
  private readonly notifications = inject(NotificationService);

  protected readonly baseZIndex = CL_Z_INDEX.toast;

  protected onClose(event: ToastCloseEvent): void {
    this.notifications.released(event.message);
  }
}
