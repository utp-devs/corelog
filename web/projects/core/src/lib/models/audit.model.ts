export interface AuditEntry {
  readonly id: number;
  readonly organizationId: string | null;
  readonly projectId: string | null;
  readonly actorId: string | null;
  readonly actorName: string;
  readonly action: string;
  readonly entityType: string;
  readonly entityId: string;
  readonly summary: string;
  readonly before: unknown;
  readonly after: unknown;
  readonly ip: string;
  readonly userAgent: string;
  readonly requestId: string;
  readonly createdAt: string;
}

export interface AuditFilters {
  readonly actor?: string;
  readonly entityType?: string;
  readonly action?: string;
  /** AAAA-MM-DD, inclusive. */
  readonly from?: string;
  readonly to?: string;
}

export interface AuditPage {
  readonly entries: readonly AuditEntry[];
  readonly nextCursor: string | null;
}

/** Where the log is read from: one organization or the whole platform. */
export type AuditScope =
  | { readonly kind: 'organization'; readonly organizationId: string }
  | { readonly kind: 'platform' };
