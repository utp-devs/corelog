import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { EmailOutboxPage } from '../../models/email-outbox.model';
import type { ScheduledJob } from '../../models/scheduled-job.model';
import { EmailOutboxApiService } from './email-outbox-api.service';
import { ScheduledJobApiService } from './scheduled-job-api.service';

function setup(): { backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return { backend: TestBed.inject(HttpTestingController) };
}

describe('EmailOutboxApiService', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists a page with the status filter and maps the rows', () => {
    const { backend } = setup();
    let page: EmailOutboxPage | undefined;
    TestBed.inject(EmailOutboxApiService)
      .list({ status: 'dead', page: 2, pageSize: 25 })
      .subscribe((value) => (page = value));
    const request = backend.expectOne(
      (r) => r.url === '/api/v1/admin/email-outbox' && r.params.get('status') === 'dead',
    );
    expect(request.request.params.get('page')).toBe('2');
    expect(request.request.params.get('page_size')).toBe('25');
    request.flush({
      data: [
        {
          id: 'e-1',
          to: 'a***@corelog.test',
          kind: 'digest',
          subject: 'Resumen',
          status: 'dead',
          attempts: 6,
          last_error: 'smtp down',
          created_at: '2026-11-27T10:00:00Z',
        },
      ],
      meta: { page: 2, page_size: 25, total: 31 },
    });
    expect(page).toEqual({
      items: [
        {
          id: 'e-1',
          userId: null,
          to: 'a***@corelog.test',
          kind: 'digest',
          subject: 'Resumen',
          status: 'dead',
          attempts: 6,
          nextAttemptAt: null,
          lastError: 'smtp down',
          createdAt: '2026-11-27T10:00:00Z',
          sentAt: null,
        },
      ],
      page: 2,
      pageSize: 25,
      total: 31,
    });
    backend.verify();
  });

  it('retries a dead email with a POST', () => {
    const { backend } = setup();
    TestBed.inject(EmailOutboxApiService).retry('e-1').subscribe();
    const request = backend.expectOne('/api/v1/admin/email-outbox/e-1/retry');
    expect(request.request.method).toBe('POST');
    request.flush(null, { status: 204, statusText: 'No Content' });
    backend.verify();
  });
});

describe('ScheduledJobApiService', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('maps jobs and their last run', () => {
    const { backend } = setup();
    let jobs: ScheduledJob[] | undefined;
    TestBed.inject(ScheduledJobApiService)
      .list()
      .subscribe((value) => (jobs = value));
    backend.expectOne('/api/v1/admin/jobs').flush({
      data: [
        {
          name: 'email-dispatch',
          schedule: 'every 10s',
          registered_at: '2026-11-27T09:00:00Z',
          last_seen_at: '2026-11-27T10:00:00Z',
          last_success_at: '2026-11-27T10:00:00Z',
          last_run: {
            status: 'ok',
            started_at: '2026-11-27T10:00:00Z',
            finished_at: '2026-11-27T10:00:01Z',
            duration_ms: 1000,
          },
        },
        {
          name: 'digest',
          schedule: 'every 24h',
          registered_at: '2026-11-27T09:00:00Z',
          last_seen_at: '2026-11-27T10:00:00Z',
          last_run: null,
        },
      ],
    });
    expect(jobs).toEqual([
      {
        name: 'email-dispatch',
        schedule: 'every 10s',
        registeredAt: '2026-11-27T09:00:00Z',
        lastSeenAt: '2026-11-27T10:00:00Z',
        lastSuccessAt: '2026-11-27T10:00:00Z',
        lastRun: {
          status: 'ok',
          startedAt: '2026-11-27T10:00:00Z',
          finishedAt: '2026-11-27T10:00:01Z',
          durationMs: 1000,
          error: '',
        },
      },
      {
        name: 'digest',
        schedule: 'every 24h',
        registeredAt: '2026-11-27T09:00:00Z',
        lastSeenAt: '2026-11-27T10:00:00Z',
        lastSuccessAt: null,
        lastRun: null,
      },
    ]);
    backend.verify();
  });
});
