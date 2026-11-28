import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { publicRequest, SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import type { ResetPasswordPayload } from '../../models/password-reset.model';

/** Forgotten password, reset through the emailed link and email verification. */
@Injectable({ providedIn: 'root' })
export class PasswordResetApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  /** Always answers the same way: the server never reveals whether the address has an account. */
  forgot(email: string): Observable<void> {
    return this.http
      .post<unknown>(
        `${this.baseUrl}/auth/password/forgot`,
        { email },
        { context: publicRequest() },
      )
      .pipe(map(() => undefined));
  }

  reset(payload: ResetPasswordPayload): Observable<void> {
    return this.http.post<void>(
      `${this.baseUrl}/auth/password/reset`,
      {
        token: payload.token,
        password: payload.password,
        password_confirmation: payload.passwordConfirmation,
      },
      { context: publicRequest() },
    );
  }

  /** Asks for a new verification email for the signed-in account. */
  requestVerification(): Observable<void> {
    return this.http
      .post<unknown>(
        `${this.baseUrl}/auth/email/verify/request`,
        {},
        { context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true) },
      )
      .pipe(map(() => undefined));
  }

  verify(token: string): Observable<void> {
    return this.http.post<void>(
      `${this.baseUrl}/auth/email/verify`,
      { token },
      { context: publicRequest() },
    );
  }
}
