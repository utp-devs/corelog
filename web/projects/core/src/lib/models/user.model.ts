/** Platform role. The first account of an installation is admin; every other account is user. */
export type AppRole = 'admin' | 'user';

/** Signed-in account as returned by the API (camelCase, defaults applied). */
export interface User {
  readonly id: string;
  readonly name: string;
  readonly email: string;
  readonly role: AppRole;
  readonly teamId: string | null;
  readonly displayName: string;
  readonly jobTitle: string;
  readonly department: string;
  readonly avatarKey: string | null;
  /** IANA time zone name, for example America/Bogota. Defaults to UTC. */
  readonly timezone: string;
  readonly createdAt: string;
  readonly lastLoginAt: string | null;
  readonly emailVerifiedAt: string | null;
}

/** Partial update of the account: omitted fields keep their current value. */
export interface UpdateProfilePayload {
  readonly name?: string;
  readonly email?: string;
  readonly displayName?: string;
  readonly jobTitle?: string;
  readonly department?: string;
  readonly timezone?: string;
}

/** Profile fields editable by the member themselves (the email is changed elsewhere). */
export type ProfileUpdate = Omit<UpdateProfilePayload, 'email'>;

export interface ChangePasswordPayload {
  readonly currentPassword: string;
  readonly newPassword: string;
}

/** Search of the platform user directory (administration). Omitted fields are not sent. */
export interface AdminSearchQuery {
  readonly query: string;
  readonly teamId?: string;
}
