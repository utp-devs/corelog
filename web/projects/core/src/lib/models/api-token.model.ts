/** Scope of a personal access token: read-only or read and write. */
export type ApiTokenScope = 'read' | 'write' | 'admin';

/** A personal access token as the list shows it. The secret is never part of it. */
export interface ApiToken {
  readonly id: string;
  readonly name: string;
  /** First characters of the token, enough to recognise it. */
  readonly prefix: string;
  readonly scopes: readonly ApiTokenScope[];
  readonly expiresAt: string | null;
  readonly lastUsedAt: string | null;
  readonly revokedAt: string | null;
  readonly createdAt: string;
}

/** Answer of the creation: the stored token plus its plain value, which exists only in this answer. */
export interface CreatedApiToken {
  readonly token: ApiToken;
  readonly secret: string;
}

export interface ApiTokenDraft {
  readonly name: string;
  readonly scopes: readonly ApiTokenScope[];
  readonly expiresAt?: string | null;
}
