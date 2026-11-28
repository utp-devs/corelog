/** Slugs of the legal documents the platform publishes. Other slugs may appear and are rendered as-is. */
export type LegalSlug =
  'privacy_policy' | 'data_processing' | 'terms_of_use' | 'cookies' | 'communications';

/** Where an acceptance was given. */
export type ConsentSource = 'register' | 'invite' | 'login_reprompt' | 'profile';

/** Current version of a document, without its body. */
export interface LegalDocumentSummary {
  readonly slug: string;
  readonly title: string;
  readonly version: number;
  readonly summary: string;
  readonly required: boolean;
  readonly effectiveFrom: string;
}

/** A version of a document with its Markdown body (empty in the administration list). */
export interface LegalDocument extends LegalDocumentSummary {
  readonly bodyMarkdown: string;
  readonly publishedAt: string | null;
}

/** Version of a document a user accepts. */
export interface ConsentRef {
  readonly slug: string;
  readonly version: number;
}

/** Required document whose current version the user has not accepted yet. */
export interface PendingConsent {
  readonly slug: string;
  readonly title: string;
  readonly version: number;
}

export interface AcceptedConsent extends ConsentRef {
  readonly acceptedAt: string;
}

export interface ConsentStatus {
  readonly accepted: readonly AcceptedConsent[];
  readonly pending: readonly PendingConsent[];
}

export interface ConsentRecord {
  readonly id: string;
  readonly userId: string;
  readonly slug: string;
  readonly version: number;
  readonly acceptedAt: string;
  readonly source: ConsentSource;
  readonly revokedAt: string | null;
}

export interface PublishLegalPayload {
  readonly slug: string;
  readonly title: string;
  readonly summary: string;
  readonly bodyMarkdown: string;
  readonly required: boolean;
}

/** Slugs the registration form asks to accept, in display order. */
export const REQUIRED_CONSENT_SLUGS = [
  'privacy_policy',
  'data_processing',
  'terms_of_use',
] as const;
export const COMMUNICATIONS_SLUG = 'communications';
