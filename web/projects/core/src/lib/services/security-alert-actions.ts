import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';

import type { NotificationAction, NotificationData } from '../models/notification.model';
import { AccountSecurityApiService } from './api/account-security-api.service';
import { NotificationService } from './notification.service';

/** Page with the sessions and the password of the account. */
export const SECURITY_PAGE = '/profile/security';

/** The two actions of a new-login alert. */
export const NOT_ME_LABEL = 'No fui yo';
export const VIEW_SESSIONS_LABEL = 'Ver mis sesiones';

/** What the user can do about a new-login alert, shared by the alert banner and the notification list. */
@Injectable({ providedIn: 'root' })
export class SecurityAlertActions {
  private readonly accountApi = inject(AccountSecurityApiService);
  private readonly router = inject(Router);
  private readonly notifications = inject(NotificationService);

  /** The actions the server attached, or the two standard ones when it sent none. */
  actionsOf(data: NotificationData | null | undefined): readonly NotificationAction[] {
    if (data?.actions?.length) {
      return data.actions;
    }
    return [
      { label: NOT_ME_LABEL, method: 'DELETE' },
      { label: VIEW_SESSIONS_LABEL, link: SECURITY_PAGE },
    ];
  }

  /** True for the action that closes the unknown session. */
  isNotMe(action: NotificationAction): boolean {
    return action.method?.toUpperCase() === 'DELETE';
  }

  /** Closes the session that was just opened. Returns false (and shows the error) when it fails. */
  async notMe(sessionId: string | null | undefined): Promise<boolean> {
    if (!sessionId) {
      return false;
    }
    try {
      await firstValueFrom(this.accountApi.revokeSession(sessionId));
      return true;
    } catch (error) {
      this.notifications.error(error);
      return false;
    }
  }

  /** Goes to the sessions page, or to the link the action carries. */
  open(link?: string): void {
    void this.router.navigateByUrl(link || SECURITY_PAGE);
  }

  /** Closes the unknown session and, on success, confirms and sends the user to change the password. */
  async notMeAndSecure(sessionId: string | null | undefined): Promise<void> {
    if (await this.notMe(sessionId)) {
      this.notifications.success(
        'Cerramos la sesión desconocida. Te recomendamos cambiar tu contraseña.',
      );
      this.open();
    }
  }
}
