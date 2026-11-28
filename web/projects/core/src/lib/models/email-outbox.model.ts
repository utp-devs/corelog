export type EmailOutboxStatus = 'pending' | 'sent' | 'dead';

/** Queued outgoing email. The address is already masked by the API. */
export interface EmailOutboxEntry {
  readonly id: string;
  readonly userId: string | null;
  readonly to: string;
  readonly kind: string;
  readonly subject: string;
  readonly status: EmailOutboxStatus;
  readonly attempts: number;
  readonly nextAttemptAt: string | null;
  readonly lastError: string;
  readonly createdAt: string;
  readonly sentAt: string | null;
}

export interface EmailOutboxPage {
  readonly items: readonly EmailOutboxEntry[];
  readonly page: number;
  readonly pageSize: number;
  readonly total: number;
}
