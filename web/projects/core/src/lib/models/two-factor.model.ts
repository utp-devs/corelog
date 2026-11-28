/** Answer of the sign-in when the account has two-step verification: the code is still missing. */
export interface TwoFactorChallenge {
  readonly twoFactorRequired: true;
  /** Short-lived token that only serves to finish the sign-in with a code. */
  readonly challengeToken: string;
}

export function isTwoFactorChallenge(value: unknown): value is TwoFactorChallenge {
  return (
    typeof value === 'object' &&
    value !== null &&
    (value as { twoFactorRequired?: unknown }).twoFactorRequired === true &&
    typeof (value as { challengeToken?: unknown }).challengeToken === 'string'
  );
}

/** Second step of the sign-in: exactly one of `code` or `recoveryCode`. */
export interface TwoFactorLoginPayload {
  readonly challengeToken: string;
  readonly code?: string;
  readonly recoveryCode?: string;
}

export interface TwoFactorStatus {
  readonly enabled: boolean;
  readonly enabledAt: string | null;
  readonly recoveryCodesRemaining: number;
}

/** Secret and otpauth address of a pending enrollment. The server shows them only once. */
export interface TwoFactorEnrollment {
  readonly secret: string;
  readonly otpauthUri: string;
}

/** Turning verification off needs the password and a code from the app or a recovery code. */
export interface TwoFactorDisablePayload {
  readonly password: string;
  readonly code?: string;
  readonly recoveryCode?: string;
}
