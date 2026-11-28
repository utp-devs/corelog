import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  ActivityGadgetItem,
  Dashboard,
  DashboardAccess,
  DashboardCreateInput,
  DashboardGadget,
  DashboardGadgetInput,
  DashboardLayout,
  DashboardListQuery,
  DashboardPage,
  DashboardPatch,
  DashboardShare,
  DashboardShareInput,
  GadgetData,
  GadgetDataEnvelope,
  GadgetType,
  StatGroup,
} from '../../models/dashboard-v2.model';
import { silentRequest } from '../../utils/silent-request';

interface GadgetDto {
  id: string;
  type: GadgetType;
  title: string;
  config: Record<string, unknown> | null;
  position: number;
  column_index: number;
}

interface ShareDto {
  type: DashboardShare['type'];
  target_id: string;
  target_name: string;
  permission: DashboardShare['permission'];
}

interface DashboardDto {
  id: string;
  name: string;
  description: string;
  layout: DashboardLayout;
  is_default: boolean;
  owner: { id: string; name: string };
  organization_id: string;
  access: DashboardAccess;
  can_edit: boolean;
  can_manage: boolean;
  is_favorite: boolean;
  shared: boolean;
  gadget_count: number;
  gadgets?: GadgetDto[];
  shares?: ShareDto[];
  updated_at: string;
}

/* eslint-disable @typescript-eslint/no-explicit-any */
type Raw = Record<string, any>;

function toGadget(dto: GadgetDto): DashboardGadget {
  return {
    id: dto.id,
    type: dto.type,
    title: dto.title,
    config: dto.config ?? {},
    position: dto.position,
    columnIndex: dto.column_index,
  };
}

export function toDashboard(dto: DashboardDto): Dashboard {
  const gadgets = (dto.gadgets ?? []).map(toGadget);
  return {
    id: dto.id,
    name: dto.name,
    description: dto.description ?? '',
    layout: dto.layout,
    isDefault: dto.is_default,
    ownerId: dto.owner.id,
    ownerName: dto.owner.name,
    organizationId: dto.organization_id,
    access: dto.access,
    canEdit: dto.can_edit,
    canManage: dto.can_manage,
    isFavorite: dto.is_favorite,
    shared: dto.shared,
    gadgetCount: dto.gadget_count ?? gadgets.length,
    gadgets,
    shares: dto.shares
      ? dto.shares.map((share) => ({
          type: share.type,
          targetId: share.target_id,
          targetName: share.target_name,
          permission: share.permission,
        }))
      : null,
    updatedAt: dto.updated_at,
  };
}

function toGroups(list: Raw[] | null | undefined): StatGroup[] {
  return (list ?? []).map((g) => ({ key: g['key'], label: g['label'], count: g['count'] }));
}

function toActivity(item: Raw): ActivityGadgetItem {
  return {
    id: item['id'],
    issueId: item['issue_id'],
    issueKey: item['issue_key'],
    summary: item['summary'],
    actor: item['actor'],
    action: item['action'],
    field: item['field'] ?? '',
    oldValue: item['old_value'] ?? '',
    newValue: item['new_value'] ?? '',
    occurredAt: item['occurred_at'],
  };
}

function toIssueList(data: Raw) {
  return {
    total: data['total'] ?? 0,
    columns: (data['columns'] ?? []) as string[],
    query: data['query'] ?? '',
    items: ((data['items'] ?? []) as Raw[]).map((row) => ({
      id: row['id'],
      key: row['key'],
      summary: row['summary'],
      status: row['status'] ?? '',
      statusCategory: row['status_category'] ?? '',
      priority: row['priority'] ?? '',
      type: row['type'] ?? '',
      assignee: row['assignee'] ?? '',
      projectKey: row['project_key'] ?? '',
      due: row['due'] ?? null,
      updated: row['updated'] ?? '',
    })),
  };
}

function toStats(data: Raw) {
  return {
    groupBy: data['group_by'] ?? '',
    total: data['total'] ?? 0,
    groups: toGroups(data['groups']),
    other: data['other'] ?? 0,
  };
}

/** Maps the snake_case data of a gadget to the model; the shape follows the gadget type. */
export function toGadgetData(type: GadgetType, data: Raw): GadgetData {
  switch (type) {
    case 'assigned_to_me':
    case 'created_by_me':
    case 'filter_results':
      return { type, data: toIssueList(data) };
    case 'issue_statistics':
    case 'pie':
      return { type, data: toStats(data) };
    case 'two_dimensional':
      return {
        type,
        data: {
          xAxis: data['x_axis'] ?? '',
          yAxis: data['y_axis'] ?? '',
          total: data['total'] ?? 0,
          xKeys: toGroups(data['x']),
          yKeys: toGroups(data['y']),
          cells: ((data['cells'] ?? []) as Raw[]).map((c) => ({
            x: c['x'],
            y: c['y'],
            count: c['count'],
          })),
        },
      };
    case 'created_vs_resolved':
      return {
        type,
        data: {
          days: data['days'] ?? 0,
          series: ((data['series'] ?? []) as Raw[]).map((p) => ({
            date: p['date'],
            created: p['created'],
            resolved: p['resolved'],
          })),
          totalCreated: data['total_created'] ?? 0,
          totalResolved: data['total_resolved'] ?? 0,
          netOpenChange: data['net_open_change'] ?? 0,
        },
      };
    case 'sprint_burndown': {
      const sprint = data['sprint'] as Raw | null;
      return {
        type,
        data: {
          sprint: sprint
            ? {
                id: sprint['id'],
                name: sprint['name'],
                projectKey: sprint['project_key'],
                startDate: sprint['start_date'] ?? null,
                endDate: sprint['end_date'] ?? null,
              }
            : null,
          totalPoints: data['total_points'] ?? 0,
          points: ((data['points'] ?? []) as Raw[]).map((p) => ({
            date: p['date'],
            remaining: p['remaining'] ?? null,
            ideal: p['ideal'],
          })),
        },
      };
    }
    case 'activity_stream':
      return { type, data: { items: ((data['items'] ?? []) as Raw[]).map(toActivity) } };
    case 'recent_projects':
      return {
        type,
        data: {
          items: ((data['items'] ?? []) as Raw[]).map((p) => ({
            id: p['id'],
            key: p['key'],
            name: p['name'],
            color: p['color'] ?? '',
            viewedAt: p['viewed_at'],
          })),
        },
      };
    case 'favorites':
      return {
        type,
        data: {
          items: ((data['items'] ?? []) as Raw[]).map((f) => ({
            type: f['type'],
            id: f['id'],
            name: f['name'],
            link: f['link'] ?? '',
            starredAt: f['starred_at'],
          })),
        },
      };
    case 'text':
      return { type, data: { markdown: data['markdown'] ?? '' } };
    case 'calendar_upcoming':
      return {
        type,
        data: {
          from: data['from'],
          to: data['to'],
          items: ((data['items'] ?? []) as Raw[]).map((i) => ({
            kind: i['kind'],
            title: i['title'],
            startsAt: i['starts_at'],
            endsAt: i['ends_at'],
            allDay: i['all_day'] ?? false,
            link: i['link'] ?? '',
            projectKey: i['project_key'] ?? '',
          })),
        },
      };
  }
}

function gadgetBody(gadget: DashboardGadgetInput): Record<string, unknown> {
  const body: Record<string, unknown> = {
    type: gadget.type,
    title: gadget.title,
    config: gadget.config,
    column_index: gadget.columnIndex,
    position: gadget.position,
  };
  if (gadget.id) body['id'] = gadget.id;
  return body;
}

/** Dashboards with gadgets: list, detail, edition, shares and the data of each gadget. */
@Injectable({ providedIn: 'root' })
export class DashboardV2ApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(query: DashboardListQuery = {}): Observable<DashboardPage> {
    let params = new HttpParams();
    if (query.scope) params = params.set('scope', query.scope);
    if (query.q) params = params.set('q', query.q);
    if (query.page) params = params.set('page', query.page);
    if (query.pageSize) params = params.set('page_size', query.pageSize);
    return this.http
      .get<{
        data: DashboardDto[];
        meta?: { page: number; page_size: number; total: number };
      }>(`${this.baseUrl}/dashboards`, { params, context: silentRequest() })
      .pipe(
        map((response) => ({
          items: response.data.map(toDashboard),
          total: response.meta?.total ?? response.data.length,
          page: response.meta?.page ?? 1,
          pageSize: response.meta?.page_size ?? response.data.length,
        })),
      );
  }

  get(id: string): Observable<Dashboard> {
    return this.http
      .get<{ data: DashboardDto }>(`${this.baseUrl}/dashboards/${id}`, {
        context: silentRequest(),
      })
      .pipe(map((r) => toDashboard(r.data)));
  }

  /** The personal default dashboard; the API creates it with the standard gadgets on first use. */
  mine(): Observable<Dashboard> {
    return this.http
      .get<{ data: DashboardDto }>(`${this.baseUrl}/me/dashboard`, { context: silentRequest() })
      .pipe(map((r) => toDashboard(r.data)));
  }

  create(input: DashboardCreateInput): Observable<Dashboard> {
    return this.http
      .post<{ data: DashboardDto }>(`${this.baseUrl}/dashboards`, {
        name: input.name,
        description: input.description ?? '',
        ...(input.layout ? { layout: input.layout } : {}),
      })
      .pipe(map((r) => toDashboard(r.data)));
  }

  update(id: string, patch: DashboardPatch): Observable<Dashboard> {
    const body: Record<string, unknown> = {};
    if (patch.name !== undefined) body['name'] = patch.name;
    if (patch.description !== undefined) body['description'] = patch.description;
    if (patch.layout !== undefined) body['layout'] = patch.layout;
    if (patch.isDefault !== undefined) body['is_default'] = patch.isDefault;
    return this.http
      .patch<{ data: DashboardDto }>(`${this.baseUrl}/dashboards/${id}`, body)
      .pipe(map((r) => toDashboard(r.data)));
  }

  remove(id: string): Observable<void> {
    return this.http.delete(`${this.baseUrl}/dashboards/${id}`).pipe(map(() => undefined));
  }

  /** Replaces the layout and all the gadgets atomically. */
  replaceGadgets(
    id: string,
    layout: DashboardLayout,
    gadgets: readonly DashboardGadgetInput[],
  ): Observable<Dashboard> {
    return this.http
      .put<{ data: DashboardDto }>(`${this.baseUrl}/dashboards/${id}/gadgets`, {
        layout,
        gadgets: gadgets.map(gadgetBody),
      })
      .pipe(map((r) => toDashboard(r.data)));
  }

  setShares(id: string, shares: readonly DashboardShareInput[]): Observable<Dashboard> {
    return this.http
      .put<{ data: DashboardDto }>(`${this.baseUrl}/dashboards/${id}/shares`, {
        shares: shares.map((s) => ({
          type: s.type,
          target_id: s.targetId,
          permission: s.permission,
        })),
      })
      .pipe(map((r) => toDashboard(r.data)));
  }

  duplicate(id: string, name?: string): Observable<Dashboard> {
    return this.http
      .post<{ data: DashboardDto }>(
        `${this.baseUrl}/dashboards/${id}/duplicate`,
        name ? { name } : {},
      )
      .pipe(map((r) => toDashboard(r.data)));
  }

  gadgetData(
    dashboardId: string,
    gadgetId: string,
    type: GadgetType,
  ): Observable<GadgetDataEnvelope> {
    return this.http
      .get<{
        data: { gadget_id: string; generated_at: string; data: Record<string, unknown> };
      }>(`${this.baseUrl}/dashboards/${dashboardId}/gadgets/${gadgetId}/data`, {
        context: silentRequest(),
      })
      .pipe(
        map(({ data }) => ({
          gadgetId: data.gadget_id,
          generatedAt: data.generated_at,
          content: toGadgetData(type, data.data ?? {}),
        })),
      );
  }
}
