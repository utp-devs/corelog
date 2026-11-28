export type WebhookDeliveryStatus = 'pending' | 'success' | 'failed' | 'dead';

export interface Webhook {
  readonly id: string;
  readonly organizationId: string;
  readonly projectId: string | null;
  readonly name: string;
  readonly url: string;
  readonly events: readonly string[];
  readonly filterQuery: string;
  readonly enabled: boolean;
  readonly createdAt: string;
  readonly updatedAt: string;
}

/** Webhook with the signing secret returned by the creation. */
export interface CreatedWebhook {
  readonly webhook: Webhook;
  readonly secret: string;
}

export interface WebhookDraft {
  readonly name: string;
  readonly url: string;
  readonly events: readonly string[];
  readonly filterQuery: string;
  readonly projectId: string | null;
  readonly enabled: boolean;
}

export interface WebhookDelivery {
  readonly id: string;
  readonly webhookId: string;
  readonly event: string;
  readonly payload: unknown;
  readonly status: WebhookDeliveryStatus;
  readonly attempts: number;
  readonly responseStatus: number | null;
  readonly responseExcerpt: string;
  readonly durationMs: number;
  readonly nextAttemptAt: string | null;
  readonly createdAt: string;
}
