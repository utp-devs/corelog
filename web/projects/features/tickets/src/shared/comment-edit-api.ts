import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { API_URL } from '@corelog/core';
import { Observable, map } from 'rxjs';

/** Edit of an own comment. The shared issue API client has no call for it. */
@Injectable({ providedIn: 'root' })
export class CommentEditApi {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${inject(API_URL)}/issues`;

  update(issueId: string, commentId: string, body: string): Observable<void> {
    return this.http
      .patch<unknown>(`${this.baseUrl}/${issueId}/comments/${commentId}`, { body })
      .pipe(map(() => undefined));
  }
}
