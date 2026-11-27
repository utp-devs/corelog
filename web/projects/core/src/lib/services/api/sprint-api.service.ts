import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { WorkItemPriority, WorkItemStatus } from '../../models/board.model';
import type {
  BurndownPoint,
  CreateSprintInput,
  Sprint,
  SprintIssue,
  SprintStatus,
  SprintUnfinishedPolicy,
  VelocityPoint,
} from '../../models/sprint.model';
import { silentRequest } from '../../utils/silent-request';

const DEFAULT_VELOCITY_LIMIT = 5;

interface SprintDto {
  readonly id: string;
  readonly project_id: string;
  readonly name: string;
  readonly goal: string;
  readonly status: SprintStatus;
  readonly start_date?: string | null;
  readonly end_date?: string | null;
  readonly closed_at?: string | null;
}

interface SprintIssueDto {
  readonly id: string;
  readonly title: string;
  readonly status: WorkItemStatus;
  readonly priority: WorkItemPriority;
  readonly story_points?: number | null;
  readonly closed_at?: string | null;
}

interface BurndownDto {
  readonly date: string;
  readonly ideal: number;
  readonly remaining: number;
}

interface VelocityDto {
  readonly sprint_id: string;
  readonly name: string;
  readonly points: number;
}

interface DataEnvelope<T> {
  readonly data: T;
}

function toSprint(dto: SprintDto): Sprint {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    goal: dto.goal,
    status: dto.status,
    startDate: dto.start_date ?? undefined,
    endDate: dto.end_date ?? undefined,
    closedAt: dto.closed_at ?? undefined,
  };
}

function toSprintIssue(dto: SprintIssueDto): SprintIssue {
  return {
    id: dto.id,
    title: dto.title,
    status: dto.status,
    priority: dto.priority,
    storyPoints: dto.story_points ?? undefined,
    closedAt: dto.closed_at ?? undefined,
  };
}

/** Sprints of a project: lifecycle, assigned issues, burndown and velocity. */
@Injectable({ providedIn: 'root' })
export class SprintApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(projectId: string): Observable<Sprint[]> {
    return this.http
      .get<DataEnvelope<SprintDto[]>>(`${this.baseUrl}/sprints`, {
        params: { project_id: projectId },
        context: silentRequest(),
      })
      .pipe(map((response) => response.data.map(toSprint)));
  }

  /** Plans a sprint. Empty dates are sent as null. */
  create(projectId: string, input: CreateSprintInput): Observable<Sprint> {
    return this.http
      .post<DataEnvelope<SprintDto>>(
        `${this.baseUrl}/sprints`,
        {
          name: input.name,
          goal: input.goal,
          start_date: input.startDate || null,
          end_date: input.endDate || null,
        },
        { params: { project_id: projectId }, context: silentRequest() },
      )
      .pipe(map((response) => toSprint(response.data)));
  }

  start(projectId: string, sprintId: string): Observable<Sprint> {
    return this.http
      .post<DataEnvelope<SprintDto>>(
        `${this.baseUrl}/sprints/${sprintId}/start`,
        {},
        { params: { project_id: projectId }, context: silentRequest() },
      )
      .pipe(map((response) => toSprint(response.data)));
  }

  /** Closes an active sprint. `unfinished` decides where the open issues go. */
  close(
    projectId: string,
    sprintId: string,
    unfinished: SprintUnfinishedPolicy,
    targetSprintId?: string,
  ): Observable<Sprint> {
    return this.http
      .post<DataEnvelope<SprintDto>>(
        `${this.baseUrl}/sprints/${sprintId}/close`,
        { unfinished, target_sprint_id: targetSprintId ?? null },
        { params: { project_id: projectId }, context: silentRequest() },
      )
      .pipe(map((response) => toSprint(response.data)));
  }

  listIssues(projectId: string, sprintId: string): Observable<SprintIssue[]> {
    return this.http
      .get<DataEnvelope<SprintIssueDto[]>>(`${this.baseUrl}/sprints/${sprintId}/issues`, {
        params: { project_id: projectId },
        context: silentRequest(),
      })
      .pipe(map((response) => response.data.map(toSprintIssue)));
  }

  assignIssue(projectId: string, sprintId: string, issueId: string): Observable<void> {
    return this.http.put<void>(
      `${this.baseUrl}/sprints/${sprintId}/issues/${issueId}`,
      {},
      { params: { project_id: projectId }, context: silentRequest() },
    );
  }

  removeIssue(projectId: string, sprintId: string, issueId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/sprints/${sprintId}/issues/${issueId}`, {
      params: { project_id: projectId },
      context: silentRequest(),
    });
  }

  burndown(projectId: string, sprintId: string): Observable<BurndownPoint[]> {
    return this.http
      .get<DataEnvelope<BurndownDto[]>>(`${this.baseUrl}/sprints/${sprintId}/burndown`, {
        params: { project_id: projectId },
        context: silentRequest(),
      })
      .pipe(map((response) => response.data));
  }

  /** Points completed by the last closed sprints of the project, most recent first. */
  velocity(projectId: string, limit = DEFAULT_VELOCITY_LIMIT): Observable<VelocityPoint[]> {
    return this.http
      .get<DataEnvelope<VelocityDto[]>>(`${this.baseUrl}/sprints/velocity`, {
        params: { project_id: projectId, limit },
        context: silentRequest(),
      })
      .pipe(
        map((response) =>
          response.data.map((point) => ({
            sprintId: point.sprint_id,
            name: point.name,
            points: point.points,
          })),
        ),
      );
  }
}
