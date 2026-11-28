/** In-app notification of the signed-in user. */
export interface AppNotification {
  readonly id: string;
  readonly kind: string;
  readonly message: string;
  readonly link: string;
  readonly read: boolean;
  readonly createdAt: string;
  /** Structured detail some kinds carry, for example the actions of a security alert. */
  readonly data?: NotificationData;
}

/** Action offered by a notification: an API call (method and path) or a page (link). */
export interface NotificationAction {
  readonly label: string;
  readonly method?: string;
  readonly path?: string;
  readonly link?: string;
}

/** Detail of a notification. Security alerts fill all of it. */
export interface NotificationData {
  readonly title?: string;
  readonly session_id?: string;
  readonly device?: string;
  readonly browser?: string;
  readonly os?: string;
  readonly ip?: string;
  readonly at?: string;
  readonly actions?: readonly NotificationAction[];
}

/** Kind of the alert sent when the account is opened from a new device. */
export const NEW_LOGIN_KIND = 'security.new_login';

/** Shape of an item of the notification list and of a notification stream event. */
export interface NotificationDto {
  readonly id: string;
  readonly kind: string;
  readonly message: string;
  readonly link: string;
  readonly read: boolean;
  readonly created_at: string;
  readonly data?: NotificationData | null;
}

/** Security event of the stream (session event). */
export interface SessionEvent {
  readonly type: 'new_login' | 'session_revoked' | 'stream_replaced';
  readonly session_id?: string;
  readonly notification_id?: string;
  readonly message?: string;
  readonly at?: string;
  readonly data?: NotificationData | null;
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
