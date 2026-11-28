import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { Issue } from '../../models/issue.model';
import {
  BulkAction,
  BulkResult,
  CloneIssueInput,
  ConvertIssueInput,
  CustomFieldType,
  IssueChange,
  IssueChildren,
  IssueCustomField,
  IssueLinkEntry,
  IssueLinkKindV2,
  IssuePermission,
  IssueView,
  MoveIssueInput,
  MoveIssueResult,
  NamedRef,
  PatchIssueInput,
  PatchIssueResult,
  ProjectVersion,
} from '../../models/issue-view.model';
import { silentRequest } from '../../utils/silent-request';
import { toIssue } from './issue-api.service';

type Dto = Record<string, unknown>;

interface Envelope<T> {
  data: T;
}

function asDto(value: unknown): Dto {
  return (value ?? {}) as Dto;
}

function refs(value: unknown): NamedRef[] {
  return Array.isArray(value)
    ? value.map((item) => {
        const dto = asDto(item);
        return { id: String(dto['id'] ?? ''), name: String(dto['name'] ?? '') };
      })
    : [];
}

function toCustomField(value: unknown): IssueCustomField {
  const dto = asDto(value);
  const options = Array.isArray(dto['options']) ? dto['options'] : [];
  return {
    fieldId: String(dto['field_id']),
    name: String(dto['name']),
    type: String(dto['type']) as CustomFieldType,
    required: dto['required'] === true,
    options: options.map((option) => {
      const o = asDto(option);
      return {
        id: String(o['id']),
        label: String(o['label']),
        color: String(o['color'] ?? ''),
        position: Number(o['position'] ?? 0),
        archived: o['archived'] === true,
      };
    }),
  };
}

/** Maps the issue view payload (issue fields plus permissions, transitions, counters, breadcrumb). */
export function toIssueView(dto: Dto): IssueView {
  const base = toIssue(dto as unknown as Parameters<typeof toIssue>[0]);
  const counts = asDto(dto['counts']);
  const breadcrumb = asDto(dto['breadcrumb']);
  const project = asDto(breadcrumb['project']);
  const parent = breadcrumb['parent'] ? asDto(breadcrumb['parent']) : null;
  const transitions = Array.isArray(dto['available_transitions'])
    ? dto['available_transitions']
    : [];
  return {
    ...base,
    startDate: (dto['start_date'] as string | null | undefined) ?? null,
    originalEstimateSeconds: (dto['original_estimate_seconds'] as number | null) ?? null,
    remainingEstimateSeconds: (dto['remaining_estimate_seconds'] as number | null) ?? null,
    flagged: dto['flagged'] === true,
    components: refs(dto['components']),
    fixVersions: refs(dto['fix_versions']),
    affectsVersions: refs(dto['affects_versions']),
    custom: asDto(dto['custom']),
    customFields: Array.isArray(dto['custom_fields'])
      ? dto['custom_fields'].map(toCustomField)
      : [],
    permissions: (Array.isArray(dto['permissions']) ? dto['permissions'] : []) as IssuePermission[],
    availableTransitions: transitions.map((item) => {
      const t = asDto(item);
      const to = asDto(t['to']);
      return {
        id: String(t['id']),
        name: String(t['name']),
        toStatus: String(t['to_status']),
        to: {
          id: String(to['id']),
          name: String(to['name']),
          category: to['category'] as 'todo' | 'in_progress' | 'done',
        },
        requiredFields: (t['required_fields'] as string[] | null) ?? [],
        requiresComment: t['requires_comment'] === true,
        requiresResolution: t['requires_resolution'] === true,
        requiresAssignee: t['requires_assignee'] === true,
        needsScreen: t['needs_screen'] === true,
      };
    }),
    counts: {
      comments: Number(counts['comments'] ?? 0),
      attachments: Number(counts['attachments'] ?? 0),
      children: Number(counts['children'] ?? 0),
      links: Number(counts['links'] ?? 0),
      watchers: Number(counts['watchers'] ?? 0),
      votes: Number(counts['votes'] ?? 0),
    },
    breadcrumb: {
      project: {
        id: String(project['id'] ?? base.projectId),
        key: String(project['key'] ?? base.projectKey),
        name: String(project['name'] ?? ''),
      },
      parent: parent
        ? {
            id: String(parent['id']),
            key: String(parent['key']),
            summary: String(parent['summary']),
            type: String(parent['type'] ?? ''),
          }
        : null,
    },
    viewerHasVoted: dto['viewer_has_voted'] === true,
  };
}

/** Builds the JSON body of a partial update: only the keys present in the input are sent. */
export function toPatchBody(input: PatchIssueInput): Record<string, unknown> {
  const map: Record<keyof PatchIssueInput, string> = {
    summary: 'summary',
    description: 'description',
    issueTypeId: 'issue_type_id',
    priorityId: 'priority_id',
    parentId: 'parent_id',
    assigneeId: 'assignee_id',
    storyPoints: 'story_points',
    originalEstimateSeconds: 'original_estimate_seconds',
    remainingEstimateSeconds: 'remaining_estimate_seconds',
    startDate: 'start_date',
    dueDate: 'due_date',
    flagged: 'flagged',
    labels: 'labels',
    components: 'components',
    fixVersions: 'fix_versions',
    affectsVersions: 'affects_versions',
    custom: 'custom',
    expectedUpdatedAt: 'expected_updated_at',
  };
  const body: Record<string, unknown> = {};
  for (const key of Object.keys(input) as (keyof PatchIssueInput)[]) {
    if (input[key] !== undefined) body[map[key]] = input[key];
  }
  return body;
}

/** Issue view, partial updates, children, typed links, votes and the multi-issue operations. */
@Injectable({ providedIn: 'root' })
export class IssueViewApiService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(API_URL);
  private readonly baseUrl = `${this.api}/issues`;

  getView(ref: string): Observable<IssueView> {
    return this.http
      .get<Envelope<Dto>>(`${this.baseUrl}/${encodeURIComponent(ref)}`, {
        context: silentRequest(),
      })
      .pipe(map((body) => toIssueView(body.data)));
  }

  patch(id: string, input: PatchIssueInput): Observable<PatchIssueResult> {
    return this.http
      .patch<Envelope<Dto>>(`${this.baseUrl}/${id}`, toPatchBody(input), {
        context: silentRequest(),
      })
      .pipe(
        map((body) => ({
          issue: toIssue(body.data as unknown as Parameters<typeof toIssue>[0]),
          batchId: (body.data['batch_id'] as string | undefined) ?? null,
          changes: ((body.data['changes'] as Dto[] | undefined) ?? []).map(
            (change): IssueChange => ({
              field: String(change['field']),
              from: (change['from'] as string | null) ?? null,
              to: (change['to'] as string | null) ?? null,
            }),
          ),
        })),
      );
  }

  delete(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}`);
  }

  /** Label names of one issue. The issue payload does not carry them; the search endpoint does. */
  labelsOf(key: string): Observable<string[]> {
    const params = new HttpParams()
      .set('jql', `key = "${key.replace(/"/g, '')}"`)
      .set('fields', 'labels')
      .set('page_size', 1)
      .set('total', false);
    return this.http
      .get<Envelope<Dto[]>>(`${this.baseUrl}/search`, { params, context: silentRequest() })
      .pipe(map((body) => (body.data[0]?.['labels'] as string[] | undefined) ?? []));
  }

  vote(id: string, voted: boolean): Observable<void> {
    const url = `${this.baseUrl}/${id}/vote`;
    return voted ? this.http.put<void>(url, null) : this.http.delete<void>(url);
  }

  children(id: string): Observable<IssueChildren> {
    return this.http
      .get<Envelope<Dto>>(`${this.baseUrl}/${id}/children`, { context: silentRequest() })
      .pipe(
        map((body) => {
          const progress = asDto(body.data['progress']);
          const list = (body.data['children'] as Dto[] | undefined) ?? [];
          return {
            total: Number(progress['total'] ?? 0),
            done: Number(progress['done'] ?? 0),
            percent: Number(progress['percent'] ?? 0),
            children: list.map((child) => ({
              id: String(child['id']),
              key: String(child['key']),
              summary: String(child['summary']),
              type: String(child['type'] ?? ''),
              status: String(child['status'] ?? ''),
              assigneeId: (child['assignee_id'] as string | null) ?? null,
              assigneeName: (child['assignee_name'] as string | null) ?? null,
            })),
          };
        }),
      );
  }

  listLinks(id: string): Observable<IssueLinkEntry[]> {
    return this.http
      .get<Envelope<Dto[]>>(`${this.baseUrl}/${id}/links`, { context: silentRequest() })
      .pipe(map((body) => body.data.map(toLinkEntry)));
  }

  createLink(id: string, target: string, kind: IssueLinkKindV2): Observable<IssueLinkEntry> {
    return this.http
      .post<Envelope<Dto>>(`${this.baseUrl}/${id}/links`, { target, kind })
      .pipe(map((body) => toLinkEntry(body.data)));
  }

  deleteLink(id: string, linkId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}/links/${linkId}`);
  }

  clone(id: string, input: CloneIssueInput): Observable<Issue> {
    return this.http
      .post<Envelope<Dto>>(`${this.baseUrl}/${id}/clone`, {
        summary: input.summary,
        project_id: input.projectId,
        include: input.include,
      })
      .pipe(map((body) => toIssue(body.data as unknown as Parameters<typeof toIssue>[0])));
  }

  move(id: string, input: MoveIssueInput): Observable<MoveIssueResult> {
    const mapping = input.fieldMapping;
    return this.http
      .post<Envelope<Dto>>(
        `${this.baseUrl}/${id}/move`,
        {
          project_id: input.projectId,
          issue_type_id: input.issueTypeId,
          field_mapping: mapping
            ? { issue_type: mapping.issueType, priority: mapping.priority, status: mapping.status }
            : undefined,
        },
        { context: silentRequest() },
      )
      .pipe(
        map((body) => ({
          issue: toIssue(body.data as unknown as Parameters<typeof toIssue>[0]),
          warnings: (body.data['warnings'] as string[] | undefined) ?? [],
          moved: ((body.data['moved'] as Dto[] | undefined) ?? []).map((entry) => ({
            id: String(entry['id']),
            fromKey: String(entry['from_key']),
            toKey: String(entry['to_key']),
          })),
        })),
      );
  }

  convert(id: string, input: ConvertIssueInput): Observable<Issue> {
    const body: Record<string, unknown> = { issue_type_id: input.issueTypeId };
    if (input.parentId !== undefined) body['parent_id'] = input.parentId;
    return this.http
      .post<Envelope<Dto>>(`${this.baseUrl}/${id}/convert`, body, { context: silentRequest() })
      .pipe(map((response) => toIssue(response.data as unknown as Parameters<typeof toIssue>[0])));
  }

  bulk(
    issueIds: readonly string[],
    action: BulkAction,
    params?: Record<string, unknown>,
  ): Observable<BulkResult> {
    return this.http
      .post<Envelope<Dto>>(
        `${this.baseUrl}/bulk`,
        { issue_ids: issueIds, action, params },
        { context: silentRequest() },
      )
      .pipe(
        map((body) => ({
          succeeded: ((body.data['succeeded'] as Dto[] | undefined) ?? []).map((entry) =>
            toIssue(entry['issue'] as Parameters<typeof toIssue>[0]),
          ),
          failed: ((body.data['failed'] as Dto[] | undefined) ?? []).map((entry) => ({
            issue: String(entry['issue']),
            code: String(entry['code']),
            message: String(entry['message']),
          })),
        })),
      );
  }

  /** Downloads the CSV of the issues that match the list filters (text, with BOM). */
  exportCsv(filters: Readonly<Record<string, string | undefined>>): Observable<Blob> {
    let params = new HttpParams();
    for (const [key, value] of Object.entries(filters)) {
      if (value) params = params.set(key, value);
    }
    return this.http.get(`${this.baseUrl}/export`, { params, responseType: 'blob' });
  }

  listVersions(projectId: string): Observable<ProjectVersion[]> {
    return this.http
      .get<Envelope<Dto[]>>(`${this.api}/projects/${projectId}/versions`, {
        context: silentRequest(),
      })
      .pipe(
        map((body) =>
          body.data.map((item) => ({
            id: String(item['id']),
            name: String(item['name']),
            released: item['released'] === true,
            archived: item['archived'] === true,
          })),
        ),
      );
  }
}

function toLinkEntry(dto: Dto): IssueLinkEntry {
  const issue = asDto(dto['issue']);
  return {
    id: String(dto['id']),
    kind: dto['kind'] as IssueLinkKindV2,
    direction: dto['direction'] === 'inward' ? 'inward' : 'outward',
    label: String(dto['label'] ?? ''),
    issue: {
      id: String(issue['id']),
      key: String(issue['key']),
      summary: String(issue['summary'] ?? ''),
      status: String(issue['status'] ?? ''),
      type: String(issue['type'] ?? ''),
      priority: String(issue['priority'] ?? ''),
    },
  };
}
