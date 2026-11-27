import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { IssueWatcherState } from '../../models/ticket.model';
import { IssueWatcherApiService } from './issue-watcher-api.service';

describe('IssueWatcherApiService', () => {
  let service: IssueWatcherApiService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(IssueWatcherApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  it('reads the watcher state of an issue and maps it to camelCase', () => {
    let state: IssueWatcherState | undefined;
    service.getState('issue-1').subscribe((value) => (state = value));

    httpMock
      .expectOne('/api/v1/issues/issue-1/watchers')
      .flush({ data: { is_watching: true, count: 4 } });

    expect(state).toEqual({ isWatching: true, count: 4 });
  });

  it('watches and unwatches with the matching verbs', () => {
    let watching: boolean | undefined;
    service.watch('issue-1').subscribe((value) => (watching = value.isWatching));
    const post = httpMock.expectOne('/api/v1/issues/issue-1/watchers');
    expect(post.request.method).toBe('POST');
    post.flush({ data: { is_watching: true, count: 5 } });
    expect(watching).toBe(true);

    let count: number | undefined;
    service.unwatch('issue-1').subscribe((value) => (count = value.count));
    const del = httpMock.expectOne('/api/v1/issues/issue-1/watchers');
    expect(del.request.method).toBe('DELETE');
    del.flush({ data: { is_watching: false, count: 4 } });
    expect(count).toBe(4);
  });
});
