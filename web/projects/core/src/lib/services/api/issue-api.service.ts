import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { HierarchyLevel } from '../../models/catalog.model';
import {
  CreateIssueInput,
  IssueActivity,
  IssueComment,
  IssueLink,
  IssueLinkKind,
  IssueStatus,
  ListIssuesFilter,
  Issue,
  IssuePage,
  UpdateIssueDetailsInput,
  UpdateIssueMetadataInput,
} from '../../models/issue.model';
import { silentRequest } from '../../utils/silent-request';
import {
  BulkTransitionResult,
  IssueTransition,
  RunTransitionInput,
  WorkflowCategory,
} from '../../models/workflow.model';

interface IssueTypeRefDto {
  id: string;
  name: string;
  icon: string;
  color: string;
  hierarchy_level: HierarchyLevel;
}

interface PriorityRefDto {
  id: string;
  name: string;
  icon: string;
  color: string;
}

interface IssueDto {
  id: string;
  project_id: string;
  key: string;
  project_key: string;
  summary: string;
  description: string;
  status: IssueStatus;
  workflow_status?: { id: string; name: string; category: WorkflowCategory } | null;
  priority: PriorityRefDto;
  issue_type: IssueTypeRefDto;
  resolution?: { id: string; name: string } | null;
  resolved_at?: string | null;
  parent_id: string | null;
  due_date: string | null;
  story_points: number | null;
  rank: string;
  closed_at: string | null;
  reporter_id: string;
  reporter_name: string;
  assignee_id: string | null;
  assignee_name?: string | null;
  created_at: string;
  updated_at: string;
}

interface TransitionDto {
  id: string;
  name: string;
  to_status: string;
  to: { id: string; name: string; category: WorkflowCategory };
  required_fields?: string[] | null;
  requires_comment?: boolean;
  requires_resolution?: boolean;
  requires_assignee?: boolean;
  needs_screen?: boolean;
}

interface AppliedTransitionDto {
  id: string;
  name: string;
}

interface BulkResponseDto {
  succeeded: { issue: IssueDto }[];
  failed: { issue: string; code: string; message: string }[];
}

function toTransition(dto: TransitionDto): IssueTransition {
  return {
    id: dto.id,
    name: dto.name,
    toStatus: dto.to_status,
    to: { id: dto.to.id, name: dto.to.name, category: dto.to.category },
    requiredFields: dto.required_fields ?? [],
    requiresComment: dto.requires_comment === true,
    requiresResolution: dto.requires_resolution === true,
    requiresAssignee: dto.requires_assignee === true,
    needsScreen: dto.needs_screen === true,
  };
}

interface CommentDto {
  id: string;
  issue_id: string;
  author_id: string;
  body: string;
  created_at: string;
  updated_at: string;
  deleted_at: string | null;
}

interface ActivityDto {
  id: string;
  issue_id: string;
  actor_id: string;
  action: string;
  field_name: string | null;
  old_value: string | null;
  new_value: string | null;
  created_at: string;
}

interface LinkDto {
  id: string;
  source_issue_id: string;
  target_issue_id: string;
  kind: IssueLinkKind;
  created_by: string;
  created_at: string;
}

interface Envelope<T> {
  data: T;
}

interface PagedEnvelope<T> {
  data: T;
  meta: { page: number; page_size: number; total: number };
}

export function toIssue(dto: IssueDto): Issue {
  return {
    id: dto.id,
    projectId: dto.project_id,
    key: dto.key,
    projectKey: dto.project_key,
    summary: dto.summary,
    description: dto.description,
    status: dto.status,
    workflowStatus: dto.workflow_status
      ? {
          id: dto.workflow_status.id,
          name: dto.workflow_status.name,
          category: dto.workflow_status.category,
        }
      : null,
    priority: {
      id: dto.priority.id,
      name: dto.priority.name,
      icon: dto.priority.icon,
      color: dto.priority.color,
    },
    issueType: {
      id: dto.issue_type.id,
      name: dto.issue_type.name,
      icon: dto.issue_type.icon,
      color: dto.issue_type.color,
      hierarchyLevel: dto.issue_type.hierarchy_level,
    },
    resolution: dto.resolution ? { id: dto.resolution.id, name: dto.resolution.name } : null,
    resolvedAt: dto.resolved_at ?? null,
    parentId: dto.parent_id,
    dueDate: dto.due_date,
    storyPoints: dto.story_points,
    rank: dto.rank,
    closedAt: dto.closed_at,
    reporterId: dto.reporter_id,
    reporterName: dto.reporter_name,
    assigneeId: dto.assignee_id ?? null,
    assigneeName: dto.assignee_name ?? null,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

function toComment(dto: CommentDto): IssueComment {
  return {
    id: dto.id,
    issueId: dto.issue_id,
    authorId: dto.author_id,
    body: dto.body,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
    deletedAt: dto.deleted_at,
  };
}

function toActivity(dto: ActivityDto): IssueActivity {
  return {
    id: dto.id,
    issueId: dto.issue_id,
    actorId: dto.actor_id,
    action: dto.action,
    fieldName: dto.field_name,
    oldValue: dto.old_value,
    newValue: dto.new_value,
    createdAt: dto.created_at,
  };
}

function toLink(dto: LinkDto): IssueLink {
  return {
    id: dto.id,
    sourceIssueId: dto.source_issue_id,
    targetIssueId: dto.target_issue_id,
    kind: dto.kind,
    createdBy: dto.created_by,
    createdAt: dto.created_at,
  };
}

/** Builds query parameters, skipping the fields that are undefined or null. */
function toParams(
  values: Readonly<Record<string, string | number | undefined | null>>,
): HttpParams {
  let params = new HttpParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null) {
      params = params.set(key, String(value));
    }
  }
  return params;
}

@Injectable({ providedIn: 'root' })
export class IssueApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${inject(API_URL)}/issues`;

  list(filter: ListIssuesFilter = {}): Observable<IssuePage> {
    const params = toParams({
      status: filter.status,
      priority: filter.priority,
      q: filter.query,
      issue_type: filter.issueType,
      label_id: filter.labelId,
      assignee_id: filter.assigneeId,
      due_from: filter.dueFrom,
      due_to: filter.dueTo,
      page: filter.page,
      page_size: filter.pageSize,
    });
    return this.http.get<PagedEnvelope<IssueDto[]>>(this.baseUrl, { params }).pipe(
      map((body) => ({
        items: body.data.map(toIssue),
        total: body.meta.total,
        page: body.meta.page,
        pageSize: body.meta.page_size,
      })),
    );
  }

  getById(id: string): Observable<Issue> {
    return this.http
      .get<Envelope<IssueDto>>(`${this.baseUrl}/${id}`, { context: silentRequest() })
      .pipe(map((body) => toIssue(body.data)));
  }

  create(input: CreateIssueInput): Observable<Issue> {
    return this.http
      .post<Envelope<IssueDto>>(this.baseUrl, {
        project_id: input.projectId,
        summary: input.summary,
        description: input.description,
        priority_id: input.priorityId,
        issue_type_id: input.issueTypeId,
        parent_id: input.parentId,
        due_date: input.dueDate,
        story_points: input.storyPoints,
        custom: input.custom,
      })
      .pipe(map((body) => toIssue(body.data)));
  }

  /** Transitions the caller can run on the issue now, in workflow order. */
  listTransitions(id: string): Observable<IssueTransition[]> {
    return this.http
      .get<Envelope<TransitionDto[]>>(`${this.baseUrl}/${id}/transitions`, {
        context: silentRequest(),
      })
      .pipe(map((body) => body.data.map(toTransition)));
  }

  /** Runs a workflow transition. Failures come back as ApiError with the workflow codes. */
  runTransition(id: string, input: RunTransitionInput): Observable<Issue> {
    return this.http
      .post<Envelope<IssueDto & { transition?: AppliedTransitionDto }>>(
        `${this.baseUrl}/${id}/transitions`,
        {
          transition_id: input.transitionId,
          fields: input.fields,
          comment: input.comment,
          resolution_id: input.resolutionId,
          assignee_id: input.assigneeId,
          expected_status_id: input.expectedStatusId,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toIssue(body.data)));
  }

  /** Transition several issues. Exactly one of the three selectors is sent. */
  bulkTransition(
    issueIds: readonly string[],
    selector: { transitionId?: string; statusId?: string; status?: IssueStatus },
  ): Observable<BulkTransitionResult> {
    return this.http
      .post<Envelope<BulkResponseDto>>(
        `${this.baseUrl}/bulk`,
        {
          issue_ids: issueIds,
          action: 'transition',
          params: {
            transition_id: selector.transitionId,
            status_id: selector.statusId,
            status: selector.status,
          },
        },
        { context: silentRequest() },
      )
      .pipe(
        map((body) => ({
          succeeded: body.data.succeeded.map((entry) => toIssue(entry.issue)),
          failed: body.data.failed.map((entry) => ({
            issue: entry.issue,
            code: entry.code,
            message: entry.message,
          })),
        })),
      );
  }

  /** Passing null clears the assignee (the issue becomes unassigned). */
  assign(id: string, assigneeId: string | null): Observable<Issue> {
    return this.http
      .patch<Envelope<IssueDto>>(`${this.baseUrl}/${id}/assign`, { assignee_id: assigneeId })
      .pipe(map((body) => toIssue(body.data)));
  }

  updateMetadata(id: string, input: UpdateIssueMetadataInput): Observable<Issue> {
    return this.http
      .patch<Envelope<IssueDto>>(`${this.baseUrl}/${id}/metadata`, {
        issue_type_id: input.issueTypeId,
        parent_id: input.parentId,
        due_date: input.dueDate,
        story_points: input.storyPoints,
      })
      .pipe(map((body) => toIssue(body.data)));
  }

  updateDetails(id: string, input: UpdateIssueDetailsInput): Observable<Issue> {
    return this.http
      .patch<Envelope<IssueDto>>(`${this.baseUrl}/${id}/details`, {
        summary: input.summary,
        description: input.description,
        priority_id: input.priorityId,
        issue_type_id: input.issueTypeId,
        parent_id: input.parentId,
        due_date: input.dueDate,
        story_points: input.storyPoints,
      })
      .pipe(map((body) => toIssue(body.data)));
  }

  listComments(id: string): Observable<IssueComment[]> {
    return this.http
      .get<Envelope<CommentDto[]>>(`${this.baseUrl}/${id}/comments`, { context: silentRequest() })
      .pipe(map((body) => body.data.map(toComment)));
  }

  createComment(id: string, body: string): Observable<IssueComment> {
    return this.http
      .post<Envelope<CommentDto>>(`${this.baseUrl}/${id}/comments`, { body })
      .pipe(map((response) => toComment(response.data)));
  }

  /** Edits the body of a comment of the caller and returns the stored comment. */
  updateComment(issueId: string, commentId: string, body: string): Observable<IssueComment> {
    return this.http
      .patch<Envelope<CommentDto>>(`${this.baseUrl}/${issueId}/comments/${commentId}`, { body })
      .pipe(map((response) => toComment(response.data)));
  }

  deleteComment(issueId: string, commentId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${issueId}/comments/${commentId}`);
  }

  listActivities(id: string): Observable<IssueActivity[]> {
    return this.http
      .get<Envelope<ActivityDto[]>>(`${this.baseUrl}/${id}/activities`, {
        context: silentRequest(),
      })
      .pipe(map((body) => body.data.map(toActivity)));
  }

  listLinks(id: string): Observable<IssueLink[]> {
    return this.http
      .get<Envelope<LinkDto[]>>(`${this.baseUrl}/${id}/links`, { context: silentRequest() })
      .pipe(map((body) => body.data.map(toLink)));
  }

  createLink(id: string, targetIssueId: string, kind: IssueLinkKind): Observable<IssueLink> {
    return this.http
      .post<Envelope<LinkDto>>(`${this.baseUrl}/${id}/links`, {
        target_issue_id: targetIssueId,
        kind,
      })
      .pipe(map((body) => toLink(body.data)));
  }

  deleteLink(id: string, linkId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}/links/${linkId}`);
  }

  updateRank(id: string, rank: string): Observable<Issue> {
    return this.http
      .patch<Envelope<IssueDto>>(`${this.baseUrl}/${id}/rank`, { rank })
      .pipe(map((body) => toIssue(body.data)));
  }
}
