/** In-app notification of the signed-in user. */
export interface AppNotification {
  readonly id: string;
  readonly kind: string;
  readonly message: string;
  readonly link: string;
  readonly read: boolean;
  readonly createdAt: string;
}

/** Notification preferences of the signed-in user. The email channel is not available yet. */
export interface NotificationPreferences {
  readonly inApp: boolean;
}
