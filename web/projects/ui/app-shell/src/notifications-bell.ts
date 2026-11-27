import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject } from '@angular/core';
import { Router } from '@angular/router';
import { type AppNotification, NotificationStore } from '@corelog/core';
import { OverlayBadge } from 'primeng/overlaybadge';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Popover } from 'primeng/popover';

import { ShellIcon } from './shell-icon';

/** Bell with the unread counter, the notification list and the mark-all action. */
@Component({
  selector: 'app-notifications-bell',
  imports: [AppTooltip, OverlayBadge, Popover, ShellIcon],
  templateUrl: './notifications-bell.html',
  styleUrl: './notifications-bell.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NotificationsBell {
  private readonly store = inject(NotificationStore);
  private readonly router = inject(Router);

  protected readonly count = this.store.unreadCount;
  protected readonly items = this.store.items;
  protected readonly badgeValue = computed(() => {
    const count = this.count();
    return count > 99 ? '99+' : String(count);
  });
  protected readonly label = computed(() => {
    const count = this.count();
    return count > 0 ? `Notificaciones, ${count} sin leer` : 'Notificaciones';
  });

  constructor() {
    this.store.startPolling();
    inject(DestroyRef).onDestroy(() => this.store.stopPolling());
  }

  /** Loads the list every time the panel opens. */
  protected onShow(): void {
    this.store.loadList();
  }

  protected markAll(): void {
    this.store.markAllRead().subscribe({ error: () => undefined });
  }

  /** Marks the notification as read (when unread), closes the panel and opens its target. */
  protected open(item: AppNotification, panel: Popover): void {
    this.store.markRead(item).subscribe({ error: () => undefined });
    panel.hide();
    if (item.link) {
      void this.router.navigateByUrl(item.link);
    }
  }
}
