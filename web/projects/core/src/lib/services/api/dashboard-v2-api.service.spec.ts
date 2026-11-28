import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { Dashboard, DashboardPage, GadgetDataEnvelope } from '../../models/dashboard-v2.model';
import { DashboardV2ApiService, toGadgetData } from './dashboard-v2-api.service';

const BASE = '/api/v1';

function dto(overrides: Record<string, unknown> = {}) {
  return {
    id: 'd-1',
    name: 'Mi panel',
    description: '',
    layout: '2',
    is_default: true,
    owner: { id: 'u-1', name: 'Ana' },
    organization_id: 'o-1',
    access: 'owner',
    can_edit: true,
    can_manage: true,
    is_favorite: false,
    shared: false,
    gadget_count: 1,
    gadgets: [
      {
        id: 'g-1',
        type: 'pie',
        title: 'Por estado',
        config: { group_by: 'status' },
        position: 0,
        column_index: 1,
      },
    ],
    shares: [{ type: 'user', target_id: 'u-2', target_name: 'Luis', permission: 'view' }],
    updated_at: '2026-11-27T10:00:00Z',
    ...overrides,
  };
}

function setup() {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(DashboardV2ApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('DashboardV2ApiService', () => {
  it('maps a dashboard with gadgets and shares', () => {
    const { service, backend } = setup();
    let dashboard: Dashboard | undefined;
    service.get('d-1').subscribe((value) => (dashboard = value));
    backend.expectOne(`${BASE}/dashboards/d-1`).flush({ data: dto() });

    expect(dashboard).toMatchObject({
      id: 'd-1',
      isDefault: true,
      ownerName: 'Ana',
      canEdit: true,
      layout: '2',
    });
    expect(dashboard?.gadgets[0]).toEqual({
      id: 'g-1',
      type: 'pie',
      title: 'Por estado',
      config: { group_by: 'status' },
      position: 0,
      columnIndex: 1,
    });
    expect(dashboard?.shares?.[0]).toEqual({
      type: 'user',
      targetId: 'u-2',
      targetName: 'Luis',
      permission: 'view',
    });
  });

  it('keeps shares null when the API does not send them and counts gadgets of list entries', () => {
    const { service, backend } = setup();
    let page: DashboardPage | undefined;
    service.list({ scope: 'shared', q: 'eq', page: 2 }).subscribe((value) => (page = value));
    const request = backend.expectOne((r) => r.url === `${BASE}/dashboards`);
    expect(request.request.params.get('scope')).toBe('shared');
    expect(request.request.params.get('q')).toBe('eq');
    expect(request.request.params.get('page')).toBe('2');
    request.flush({
      data: [dto({ gadgets: undefined, shares: undefined, gadget_count: 4 })],
      meta: { page: 2, page_size: 20, total: 21 },
    });

    expect(page?.total).toBe(21);
    expect(page?.items[0].gadgetCount).toBe(4);
    expect(page?.items[0].shares).toBeNull();
  });

  it('loads the personal dashboard from the me endpoint', () => {
    const { service, backend } = setup();
    service.mine().subscribe();
    backend.expectOne(`${BASE}/me/dashboard`).flush({ data: dto() });
  });

  it('replaces gadgets in one request, sending ids only for stored gadgets', () => {
    const { service, backend } = setup();
    service
      .replaceGadgets('d-1', '3', [
        { id: 'g-1', type: 'pie', title: 'A', config: {}, columnIndex: 0, position: 0 },
        { type: 'text', title: 'B', config: { markdown: 'x' }, columnIndex: 2, position: 0 },
      ])
      .subscribe();
    const request = backend.expectOne(`${BASE}/dashboards/d-1/gadgets`);
    expect(request.request.method).toBe('PUT');
    expect(request.request.body).toEqual({
      layout: '3',
      gadgets: [
        { id: 'g-1', type: 'pie', title: 'A', config: {}, column_index: 0, position: 0 },
        { type: 'text', title: 'B', config: { markdown: 'x' }, column_index: 2, position: 0 },
      ],
    });
    request.flush({ data: dto() });
  });

  it('sends shares, patches and duplicates with the API field names', () => {
    const { service, backend } = setup();
    service.setShares('d-1', [{ type: 'team', targetId: 't-1', permission: 'edit' }]).subscribe();
    const shares = backend.expectOne(`${BASE}/dashboards/d-1/shares`);
    expect(shares.request.body).toEqual({
      shares: [{ type: 'team', target_id: 't-1', permission: 'edit' }],
    });
    shares.flush({ data: dto() });

    service.update('d-1', { isDefault: true, name: 'X' }).subscribe();
    const patch = backend.expectOne(`${BASE}/dashboards/d-1`);
    expect(patch.request.method).toBe('PATCH');
    expect(patch.request.body).toEqual({ name: 'X', is_default: true });
    patch.flush({ data: dto() });

    service.duplicate('d-1').subscribe();
    const copy = backend.expectOne(`${BASE}/dashboards/d-1/duplicate`);
    expect(copy.request.body).toEqual({});
    copy.flush({ data: dto({ id: 'd-2' }) });
  });

  it('maps the data of a gadget by its type', () => {
    const { service, backend } = setup();
    let envelope: GadgetDataEnvelope | undefined;
    service.gadgetData('d-1', 'g-1', 'pie').subscribe((value) => (envelope = value));
    backend.expectOne(`${BASE}/dashboards/d-1/gadgets/g-1/data`).flush({
      data: {
        gadget_id: 'g-1',
        type: 'pie',
        generated_at: '2026-11-27T10:00:00Z',
        data: {
          group_by: 'status',
          total: 3,
          groups: [{ key: 'todo', label: 'Por hacer', count: 3 }],
          other: 0,
        },
      },
    });
    expect(envelope?.content).toEqual({
      type: 'pie',
      data: {
        groupBy: 'status',
        total: 3,
        groups: [{ key: 'todo', label: 'Por hacer', count: 3 }],
        other: 0,
      },
    });
  });

  it('maps every gadget data shape', () => {
    const issues = toGadgetData('assigned_to_me', {
      total: 1,
      columns: ['key'],
      query: 'q',
      items: [
        {
          id: 'i',
          key: 'CL-1',
          summary: 's',
          status: 'Abierta',
          status_category: 'todo',
          priority: 'high',
          type: 'Tarea',
          assignee: 'Ana',
          project_key: 'CL',
          due: '2026-12-01',
          updated: '2026-11-27T10:00:00Z',
        },
      ],
    });
    expect(issues.type === 'assigned_to_me' && issues.data.items[0].statusCategory).toBe('todo');

    const burndown = toGadgetData('sprint_burndown', {
      sprint: null,
      total_points: 0,
      points: [{ date: '2026-11-27', remaining: null, ideal: 3 }],
    });
    expect(burndown.type === 'sprint_burndown' && burndown.data.points[0].remaining).toBeNull();

    const upcoming = toGadgetData('calendar_upcoming', {
      from: 'a',
      to: 'b',
      items: [{ kind: 'sprint_end', title: 'S', starts_at: 'x', ends_at: 'y', all_day: true }],
    });
    expect(upcoming.type === 'calendar_upcoming' && upcoming.data.items[0].allDay).toBe(true);

    const text = toGadgetData('text', { markdown: '# hola' });
    expect(text.type === 'text' && text.data.markdown).toBe('# hola');

    const cvr = toGadgetData('created_vs_resolved', {
      days: 7,
      series: [{ date: 'd', created: 1, resolved: 2 }],
      total_created: 1,
      total_resolved: 2,
      net_open_change: -1,
    });
    expect(cvr.type === 'created_vs_resolved' && cvr.data.netOpenChange).toBe(-1);

    const matrix = toGadgetData('two_dimensional', {
      x_axis: 'status',
      y_axis: 'assignee',
      total: 1,
      x: [{ key: 'a', label: 'A', count: 1 }],
      y: [{ key: 'b', label: 'B', count: 1 }],
      cells: [{ x: 'a', y: 'b', count: 1 }],
    });
    expect(matrix.type === 'two_dimensional' && matrix.data.cells.length).toBe(1);
  });
});
