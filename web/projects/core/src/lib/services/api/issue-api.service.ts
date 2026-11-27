import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  CreateIssueInput,
  IssueActivity,
  IssueComment,
  IssueLink,
  IssueLinkKind,
  IssuePriority,
  IssueStatus,
  IssueType,
  ListIssuesFilter,
  Issue,
  IssuePage,
  UpdateIssueDetailsInput,
  UpdateIssueMetadataInput,
} from '../../models/issue.model';
import { silentRequest } from '../../utils/silent-request';

interface IssueDto {
  id: string;
  project_id: string;
  key: string;
  project_key: string;
  summary: string;
  description: string;
  status: IssueStatus;
  priority: IssuePriority;
  issue_type: IssueType;
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

function toIssue(dto: IssueDto): Issue {
  return {
    id: dto.id,
    projectId: dto.project_id,
    key: dto.key,
    projectKey: dto.project_key,
    summary: dto.summary,
    description: dto.description,
    status: dto.status,
    priority: dto.priority,
    issueType: dto.issue_type,
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
        priority: input.priority,
        issue_type: input.issueType,
        parent_id: input.parentId,
        due_date: input.dueDate,
        story_points: input.storyPoints,
      })
      .pipe(map((body) => toIssue(body.data)));
  }

  updateStatus(id: string, status: IssueStatus): Observable<Issue> {
    return this.http
      .patch<Envelope<IssueDto>>(`${this.baseUrl}/${id}/status`, { status })
      .pipe(map((body) => toIssue(body.data)));
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
        issue_type: input.issueType,
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
        priority: input.priority,
        issue_type: input.issueType,
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
