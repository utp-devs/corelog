import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import type { PhoneVerificationRequest } from '../../models/country.model';
import type { User } from '../../models/user.model';
import { mapUser, type UserResponseDto } from './auth-api.service';

/** Verification of the phone of the signed-in account by a code. */
@Injectable({ providedIn: 'root' })
export class PhoneApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${inject(API_URL)}/users/me/phone`;
  private readonly quiet = (): HttpContext => new HttpContext().set(SKIP_ERROR_NOTIFICATION, true);

  /** Asks for a six digit code. Answers with the masked destination and when the code expires. */
  requestVerification(): Observable<PhoneVerificationRequest> {
    return this.http
      .post<{ data: { destination: string; expires_at: string } }>(
        `${this.baseUrl}/verification`,
        {},
        { context: this.quiet() },
      )
      .pipe(map((r) => ({ destination: r.data.destination, expiresAt: r.data.expires_at })));
  }

  /** Checks the code; answers with the account, whose phone is now verified. */
  verify(code: string): Observable<User> {
    return this.http
      .post<{ data: UserResponseDto }>(
        `${this.baseUrl}/verify`,
        { code },
        { context: this.quiet() },
      )
      .pipe(map((r) => mapUser(r.data)));
  }
}
