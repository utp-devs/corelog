/** In-app notification of the signed-in user. */
export interface AppNotification {
  readonly id: string;
  readonly kind: string;
  readonly message: string;
  readonly link: string;
  readonly read: boolean;
  readonly createdAt: string;
}

/** How email notices are delivered: each one at once, one daily digest, or never. */
export type EmailNotificationMode = 'immediate' | 'daily' | 'off';

/** Notification preferences of the signed-in user. */
export interface NotificationPreferences {
  readonly inApp: boolean;
  readonly email: EmailNotificationMode;
  readonly notifyOwnActions: boolean;
}

/** Fields besides the in-app flag that a preferences update may carry. */
export interface NotificationPreferencesChange {
  readonly email?: EmailNotificationMode;
  readonly notifyOwnActions?: boolean;
}
