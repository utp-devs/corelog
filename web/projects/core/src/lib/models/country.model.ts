/** Country of the phone catalog (GET /meta/countries). */
export interface Country {
  /** ISO 3166-1 alpha-2 code. */
  readonly code: string;
  readonly name: string;
  /** International prefix with the plus sign, for example +51. */
  readonly dialCode: string;
  /** National number of example, when known. */
  readonly example: string | null;
  /** Area codes of the countries that share +1. */
  readonly areas: readonly string[];
}

/** Phone as the forms handle it: country and national digits. */
export interface PhoneValue {
  readonly country: string;
  readonly number: string;
}

/** Phone of an account as the API returns it. */
export interface UserPhone extends PhoneValue {
  readonly e164: string;
  readonly verified: boolean;
}

/** Answer of the request of a verification code. */
export interface PhoneVerificationRequest {
  /** Masked destination, for example +51******678. */
  readonly destination: string;
  readonly expiresAt: string;
}
