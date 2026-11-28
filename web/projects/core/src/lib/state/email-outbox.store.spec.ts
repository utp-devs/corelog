import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { of, throwError } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import type { EmailOutboxEntry } from '../models/email-outbox.model';
import { EmailOutboxApiService } from '../services/api/email-outbox-api.service';
import { ScheduledJobApiService } from '../services/api/scheduled-job-api.service';
import { EmailOutboxStore } from './email-outbox.store';
import { ScheduledJobStore } from './scheduled-job.store';

function entry(id: string, status: EmailOutboxEntry['status']): EmailOutboxEntry {
  return {
    id,
    userId: null,
    to: 'a***@x.test',
    kind: 'digest',
    subject: 'Resumen',
    status,
    attempts: 0,
    nextAttemptAt: null,
    lastError: '',
    createdAt: '2026-11-27T10:00:00Z',
    sentAt: null,
  };
}

function setupOutbox(api: Partial<EmailOutboxApiService>): EmailOutboxStore {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), { provide: EmailOutboxApiService, useValue: api }],
  });
  return TestBed.inject(EmailOutboxStore);
}

describe('EmailOutboxStore', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('loads a page and exposes the total and the page count', async () => {
    const list = vi.fn(() =>
      of({ items: [entry('e-1', 'pending')], page: 1, pageSize: 25, total: 60 }),
    );
    const store = setupOutbox({ list });
    await store.load();
    expect(store.items()).toHaveLength(1);
    expect(store.total()).toBe(60);
    expect(store.pageCount()).toBe(3);
    expect(store.loaded()).toBe(true);
  });

  it('goes back to the first page when the filter changes', async () => {
    const list = vi.fn(() => of({ items: [], page: 1, pageSize: 25, total: 100 }));
    const store = setupOutbox({ list });
    await store.load();
    await store.goTo(3);
    await store.setFilter('dead');
    expect(store.page()).toBe(1);
    expect(list).toHaveBeenLastCalledWith({ status: 'dead', page: 1, pageSize: 25 }, false);
  });

  it('keeps the rows when a background refresh fails', async () => {
    const list = vi
      .fn()
      .mockReturnValueOnce(of({ items: [entry('e-1', 'sent')], page: 1, pageSize: 25, total: 1 }))
      .mockReturnValueOnce(throwError(() => new HttpErrorResponse({ status: 500 })));
    const store = setupOutbox({ list });
    await store.load();
    await store.load(true);
    expect(store.items()).toHaveLength(1);
    expect(store.error()?.status).toBe(500);
  });

  it('retries a dead email and refreshes the page', async () => {
    const retry = vi.fn(() => of(undefined));
    const list = vi.fn(() => of({ items: [], page: 1, pageSize: 25, total: 0 }));
    const store = setupOutbox({ list, retry });
    expect(await store.retry('e-1')).toBeNull();
    expect(retry).toHaveBeenCalledWith('e-1');
    expect(list).toHaveBeenCalledTimes(1);
    expect(store.retrying().size).toBe(0);
  });

  it('returns the conflict of a retry', async () => {
    const store = setupOutbox({
      retry: vi.fn(() => throwError(() => new HttpErrorResponse({ status: 409 }))),
    });
    expect((await store.retry('e-1'))?.status).toBe(409);
  });
});

describe('ScheduledJobStore', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('loads the jobs and reports an empty list', async () => {
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        { provide: ScheduledJobApiService, useValue: { list: vi.fn(() => of([])) } },
      ],
    });
    const store = TestBed.inject(ScheduledJobStore);
    await store.load();
    expect(store.isEmpty()).toBe(true);
  });

  it('keeps the error of the first load', async () => {
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        {
          provide: ScheduledJobApiService,
          useValue: { list: vi.fn(() => throwError(() => new HttpErrorResponse({ status: 403 }))) },
        },
      ],
    });
    const store = TestBed.inject(ScheduledJobStore);
    await store.load();
    expect(store.error()?.status).toBe(403);
    expect(store.loaded()).toBe(false);
  });
});
