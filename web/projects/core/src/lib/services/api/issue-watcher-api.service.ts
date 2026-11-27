import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { IssueWatcherState } from '../../models/ticket.model';
import { silentRequest } from '../../utils/silent-request';

interface WatcherStateDto {
  is_watching: boolean;
  count: number;
}

interface Envelope<T> {
  data: T;
}

function toWatcherState(dto: WatcherStateDto): IssueWatcherState {
  return { isWatching: dto.is_watching, count: dto.count };
}

@Injectable({ providedIn: 'root' })
export class IssueWatcherApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${inject(API_URL)}/issues`;

  getState(issueId: string): Observable<IssueWatcherState> {
    return this.http
      .get<Envelope<WatcherStateDto>>(`${this.baseUrl}/${issueId}/watchers`, {
        context: silentRequest(),
      })
      .pipe(map((body) => toWatcherState(body.data)));
  }

  watch(issueId: string): Observable<IssueWatcherState> {
    return this.http
      .post<Envelope<WatcherStateDto>>(`${this.baseUrl}/${issueId}/watchers`, null)
      .pipe(map((body) => toWatcherState(body.data)));
  }

  unwatch(issueId: string): Observable<IssueWatcherState> {
    return this.http
      .delete<Envelope<WatcherStateDto>>(`${this.baseUrl}/${issueId}/watchers`)
      .pipe(map((body) => toWatcherState(body.data)));
  }
}
