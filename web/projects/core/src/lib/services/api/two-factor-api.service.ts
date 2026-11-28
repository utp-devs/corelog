import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION, publicRequest } from '../../config/http-context';
import type { AuthSession } from '../../models/auth.model';
import type {
  TwoFactorDisablePayload,
  TwoFactorEnrollment,
  TwoFactorLoginPayload,
  TwoFactorStatus,
} from '../../models/two-factor.model';
import { mapAuthSession, type AuthResponseDto } from './auth-api.service';

interface Envelope<T> {
  readonly data: T;
}

interface StatusDto {
  readonly enabled: boolean;
  readonly enabled_at: string | null;
  readonly recovery_codes_remaining: number;
}

interface CodesDto {
  readonly recovery_codes: string[];
}

/** Two-step verification: second step of the sign-in and the account security endpoints. */
@Injectable({ providedIn: 'root' })
export class TwoFactorApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${inject(API_URL)}/auth`;
  private readonly quiet = (): HttpContext => new HttpContext().set(SKIP_ERROR_NOTIFICATION, true);

  /** Finishes a sign-in that asked for a code. Public: the challenge token is the credential. */
  completeLogin(payload: TwoFactorLoginPayload): Observable<AuthSession> {
    const body: Record<string, string> = { challenge_token: payload.challengeToken };
    if (payload.code !== undefined) body['code'] = payload.code;
    if (payload.recoveryCode !== undefined) body['recovery_code'] = payload.recoveryCode;
    return this.http
      .post<Envelope<AuthResponseDto>>(`${this.baseUrl}/login/2fa`, body, {
        context: publicRequest(),
      })
      .pipe(map((response) => mapAuthSession(response.data)));
  }

  status(): Observable<TwoFactorStatus> {
    return this.http
      .get<Envelope<StatusDto>>(`${this.baseUrl}/2fa`, { context: this.quiet() })
      .pipe(
        map(({ data }) => ({
          enabled: data.enabled,
          enabledAt: data.enabled_at,
          recoveryCodesRemaining: data.recovery_codes_remaining,
        })),
      );
  }

  enroll(): Observable<TwoFactorEnrollment> {
    return this.http
      .post<Envelope<{ secret: string; otpauth_uri: string }>>(`${this.baseUrl}/2fa/enroll`, null, {
        context: this.quiet(),
      })
      .pipe(map(({ data }) => ({ secret: data.secret, otpauthUri: data.otpauth_uri })));
  }

  /** Confirms the enrollment with a code. Returns the recovery codes, shown once. */
  confirm(code: string): Observable<string[]> {
    return this.http
      .post<Envelope<CodesDto>>(`${this.baseUrl}/2fa/confirm`, { code }, { context: this.quiet() })
      .pipe(map(({ data }) => data.recovery_codes));
  }

  disable(payload: TwoFactorDisablePayload): Observable<void> {
    const body: Record<string, string> = { password: payload.password };
    if (payload.code !== undefined) body['code'] = payload.code;
    if (payload.recoveryCode !== undefined) body['recovery_code'] = payload.recoveryCode;
    return this.http.post<void>(`${this.baseUrl}/2fa/disable`, body, { context: this.quiet() });
  }

  /** Replaces the recovery codes (the old ones stop working). Needs a current code. */
  regenerateRecoveryCodes(code: string): Observable<string[]> {
    return this.http
      .post<Envelope<CodesDto>>(
        `${this.baseUrl}/2fa/recovery-codes`,
        { code },
        { context: this.quiet() },
      )
      .pipe(map(({ data }) => data.recovery_codes));
  }
}
