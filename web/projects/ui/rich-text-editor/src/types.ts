export interface UserLite {
  readonly id: string;
  readonly name: string;
  readonly email?: string;
}

export interface IssueLite {
  readonly key: string;
  readonly summary: string;
}
