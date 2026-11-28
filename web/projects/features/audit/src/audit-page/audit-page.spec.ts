import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import {
  AuditApiService,
  AuditEntry,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  UserDirectoryApiService,
} from '@corelog/core';
import { of, throwError } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AuditPage } from './audit-page';

function entry(id: number, patch: Partial<AuditEntry> = {}): AuditEntry {
  return {
    id,
    organizationId: 'org-1',
    projectId: null,
    actorId: 'u1',
    actorName: 'Ana',
    action: 'issue.created',
    entityType: 'issue',
    entityId: 'i1',
    summary: `Ana creó la incidencia ${id}`,
    before: null,
    after: { title: 'Nueva' },
    ip: '10.0.0.1',
    userAgent: 'Chrome',
    requestId: `req-${id}`,
    createdAt: '2026-11-25T10:00:00Z',
    ...patch,
  };
}

function setup(
  scope: 'organization' | 'platform',
  list = vi.fn((..._args: unknown[]) => of({ entries: [entry(2), entry(1)], nextCursor: 'c1' })),
) {
  const exportCsv = vi.fn(() => of(new Blob(['id'])));
  TestBed.configureTestingModule({
    imports: [AuditPage],
    providers: [
      { provide: ActivatedRoute, useValue: { snapshot: { data: { scope } } } },
      { provide: AuditApiService, useValue: { list, exportCsv } },
      { provide: ORGANIZATION_ID_PROVIDER, useValue: () => 'org-1' },
      { provide: OrganizationApiService, useValue: { listMembers: () => of([]) } },
      { provide: UserDirectoryApiService, useValue: { search: () => of([]) } },
    ],
  });
  const fixture = TestBed.createComponent(AuditPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, list, exportCsv };
}

async function settle(fixture: ComponentFixture<unknown>): Promise<void> {
  for (let pass = 0; pass < 5; pass += 1) {
    fixture.detectChanges();
    await Promise.resolve();
  }
}

describe('AuditPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the organization log, read-only', async () => {
    const { fixture, el, list } = setup('organization');
    await settle(fixture);
    expect(list.mock.calls[0][0]).toEqual({ kind: 'organization', organizationId: 'org-1' });
    expect(el.querySelectorAll('[data-testid="audit-row"]').length).toBe(2);
    expect(el.querySelector('h1')?.textContent).toContain('Auditoría');
    expect(el.querySelector('input[type="checkbox"], [data-testid="audit-delete"]')).toBeNull();
  });

  it('uses the platform endpoint for platform admins', async () => {
    const { fixture, list } = setup('platform');
    await settle(fixture);
    expect(list.mock.calls[0][0]).toEqual({ kind: 'platform' });
  });

  it('applies the action filter and starts over', async () => {
    const { fixture, el, list } = setup('organization');
    await settle(fixture);
    const action = el.querySelector<HTMLInputElement>('[data-testid="audit-action"]');
    if (action) {
      action.value = 'project.deleted';
      action.dispatchEvent(new Event('input'));
    }
    el.querySelector<HTMLElement>('[data-testid="audit-apply"] button')?.click();
    await settle(fixture);
    const last = list.mock.calls[list.mock.calls.length - 1] as unknown as [
      unknown,
      { action?: string },
    ];
    expect(last[1].action).toBe('project.deleted');
  });

  it('loads more with the cursor', async () => {
    const { fixture, el, list } = setup('organization');
    await settle(fixture);
    el.querySelector<HTMLElement>('[data-testid="audit-more"] button')?.click();
    await settle(fixture);
    const last = list.mock.calls[list.mock.calls.length - 1] as unknown as [
      unknown,
      unknown,
      { cursor?: string },
    ];
    expect(last[2].cursor).toBe('c1');
  });

  it('opens the detail drawer with the summary and the request id', async () => {
    const { fixture, el } = setup('organization');
    await settle(fixture);
    el.querySelector<HTMLButtonElement>('[data-testid="audit-open"]')?.click();
    await settle(fixture);
    expect(el.querySelector('[data-testid="audit-summary"]')?.textContent).toContain(
      'incidencia 2',
    );
    expect(el.querySelector('[data-testid="audit-request-id"]')?.textContent).toContain('req-2');
    expect(el.querySelector('[data-testid="audit-diff"]')?.textContent).toContain('Nueva');
  });

  it('shows the access state on 403', async () => {
    const forbidden = vi.fn(() =>
      throwError(() => ({ status: 403, code: 'http.403', message: 'no', details: [] })),
    );
    const { fixture, el } = setup('organization', forbidden as never);
    await settle(fixture);
    expect(el.textContent).toContain('No tienes acceso al registro de auditoría');
    expect(el.querySelector('[data-testid="audit-table"]')).toBeNull();
  });

  it('shows the empty state', async () => {
    const empty = vi.fn(() => of({ entries: [], nextCursor: null }));
    const { fixture, el } = setup('organization', empty as never);
    await settle(fixture);
    expect(el.querySelector('[data-testid="audit-empty"]')).not.toBeNull();
  });

  it('downloads the CSV with the same filters', async () => {
    const { fixture, el, exportCsv } = setup('organization');
    await settle(fixture);
    const create = vi.fn(() => 'blob:x');
    URL.createObjectURL = create;
    URL.revokeObjectURL = vi.fn();
    el.querySelector<HTMLElement>('[data-testid="audit-export"] button')?.click();
    await settle(fixture);
    expect(exportCsv).toHaveBeenCalled();
    expect(create).toHaveBeenCalled();
  });
});
