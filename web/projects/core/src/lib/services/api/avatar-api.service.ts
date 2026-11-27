import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, catchError, map, of, throwError } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import { asApiError } from '../../utils/api-error';

interface AvatarUploadResponseDto {
  readonly data: { readonly avatar_url: string };
}

/** Avatar images of the users. The image needs the bearer token, so it is fetched as a blob, never by <img src>. */
@Injectable({ providedIn: 'root' })
export class AvatarApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  /** PNG avatar of any user. Emits null when the user has no avatar (404). */
  getAvatar(userId: string): Observable<Blob | null> {
    return this.http
      .get(`${this.baseUrl}/users/${userId}/avatar`, {
        responseType: 'blob',
        context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true),
      })
      .pipe(
        catchError((error: unknown) =>
          asApiError(error).status === 404 ? of(null) : throwError(() => error),
        ),
      );
  }

  /** Replaces the caller's avatar. Emits the relative avatar URL. Size and type errors are shown by the caller. */
  uploadMyAvatar(file: File): Observable<string> {
    const form = new FormData();
    form.append('file', file);
    return this.http
      .put<AvatarUploadResponseDto>(`${this.baseUrl}/users/me/avatar`, form, {
        context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true),
      })
      .pipe(map((response) => response.data.avatar_url));
  }

  deleteMyAvatar(): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/users/me/avatar`);
  }
}
