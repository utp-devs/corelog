import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { AuditStore } from './audit.store';

function row(id: number) {
  return { id, action: 'issue.created', entity_type: 'issue', created_at: '2026-11-25T10:00:00Z' };
}

describe('AuditStore', () => {
  function setup() {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    return { store: TestBed.inject(AuditStore), backend: TestBed.inject(HttpTestingController) };
  }

  it('loads the organization log and pages with the cursor', async () => {
    const { store, backend } = setup();
    const opened = store.open(
      { kind: 'organization', organizationId: 'o1' },
      { action: 'issue.created' },
    );
    const first = backend.expectOne((r) => r.url.endsWith('/organizations/o1/audit'));
    expect(first.request.params.get('action')).toBe('issue.created');
    first.flush({ data: [row(2)], meta: { next_cursor: 'abc' } });
    await opened;
    expect(store.entries().length).toBe(1);
    expect(store.hasMore()).toBe(true);

    const more = store.loadMore();
    const second = backend.expectOne((r) => r.url.endsWith('/audit'));
    expect(second.request.params.get('cursor')).toBe('abc');
    second.flush({ data: [row(1)], meta: { next_cursor: null } });
    await more;
    expect(store.entries().map((e) => e.id)).toEqual([2, 1]);
    expect(store.hasMore()).toBe(false);
  });

  it('uses the platform endpoint and keeps the error', async () => {
    const { store, backend } = setup();
    const opened = store.open({ kind: 'platform' });
    backend
      .expectOne((r) => r.url.endsWith('/admin/audit'))
      .flush({ error: { message: 'no' } }, { status: 403, statusText: 'Forbidden' });
    await opened;
    expect(store.error()?.status).toBe(403);
    expect(store.isEmpty()).toBe(false);
  });
});
