export interface UserLite {
  readonly id: string;
  readonly name: string;
  readonly email?: string;
  /** Profile photo address; initials are shown when missing. */
  readonly avatarUrl?: string | null;
}

export interface IssueLite {
  readonly key: string;
  readonly summary: string;
}
