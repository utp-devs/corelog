export interface ResetPasswordPayload {
  readonly token: string;
  readonly password: string;
  readonly passwordConfirmation: string;
}
