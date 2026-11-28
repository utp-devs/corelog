import type { PhoneValue } from './country.model';
import type { ConsentRef } from './legal.model';
import type { User } from './user.model';

export interface LoginPayload {
  readonly email: string;
  readonly password: string;
}

export interface RegisterPayload {
  readonly firstName: string;
  readonly lastName: string;
  readonly phone: PhoneValue;
  readonly passwordConfirmation: string;
  readonly locale?: string;
  readonly consents: readonly ConsentRef[];
  readonly email: string;
  readonly password: string;
}

/** Result of a successful login: the bearer token and the account it belongs to. */
export interface AuthSession {
  readonly token: string;
  readonly user: User;
}

/** Login session of the signed-in account. The API marks the one used by the current request. */
export interface AccountSession {
  readonly id: string;
  readonly userAgent: string;
  readonly createdAt: string;
  readonly lastSeenAt: string;
  readonly current: boolean;
}
