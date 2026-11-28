import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { StatusCategory } from '../../models/board-v2.model';
import type { SprintStatus } from '../../models/sprint.model';
import type {
  AgileSprint,
  CompleteSprintInput,
  CreateAgileSprintInput,
  SprintCompletion,
  SprintReport,
  SprintReportIssue,
  StartSprintInput,
  UpdateAgileSprintInput,
} from '../../models/sprint-v2.model';
import { silentRequest } from '../../utils/silent-request';

interface Envelope<T> {
  readonly data: T;
}

export interface AgileSprintDto {
  id: string;
  project_id: string;
  board_id?: string | null;
  name: string;
  goal?: string;
  status: SprintStatus;
  start_date?: string | null;
  end_date?: string | null;
  started_at?: string | null;
  completed_at?: string | null;
  committed_points?: number | null;
  completed_points?: number | null;
}

interface ReportIssueDto {
  issue_id: string;
  key: string;
  summary: string;
  points?: number | null;
  status_category: StatusCategory;
  added_after_start?: boolean;
  removed_at?: string;
  removed_kind?: 'removed' | 'moved_out';
}

interface ReportDto {
  sprint: AgileSprintDto;
  committed_points: number;
  completed_points: number;
  incomplete_points: number;
  added_points: number;
  removed_points: number;
  completed?: ReportIssueDto[] | null;
  incomplete?: ReportIssueDto[] | null;
  removed?: ReportIssueDto[] | null;
  added_after_start?: ReportIssueDto[] | null;
  burndown?:
    | { date: string; ideal?: number | null; remaining: number; scope: number; completed: number }[]
    | null;
  velocity?: { sprint_id: string; name: string; points: number }[] | null;
}

interface CompletionDto {
  sprint: AgileSprintDto;
  next_sprint?: AgileSprintDto | null;
  completed_issues: number;
  moved_issues: number;
  completed_points: number;
  committed_points: number;
}

export function toAgileSprint(dto: AgileSprintDto): AgileSprint {
  return {
    id: dto.id,
    projectId: dto.project_id,
    boardId: dto.board_id ?? null,
    name: dto.name,
    goal: dto.goal ?? '',
    status: dto.status,
    startDate: dto.start_date ?? null,
    endDate: dto.end_date ?? null,
    startedAt: dto.started_at ?? null,
    completedAt: dto.completed_at ?? null,
    committedPoints: dto.committed_points ?? null,
    completedPoints: dto.completed_points ?? null,
  };
}

function toReportIssue(dto: ReportIssueDto): SprintReportIssue {
  return {
    issueId: dto.issue_id,
    key: dto.key,
    summary: dto.summary,
    points: dto.points ?? null,
    statusCategory: dto.status_category,
    addedAfterStart: dto.added_after_start === true,
    removedAt: dto.removed_at,
    removedKind: dto.removed_kind,
  };
}

function toReport(dto: ReportDto): SprintReport {
  return {
    sprint: toAgileSprint(dto.sprint),
    committedPoints: dto.committed_points,
    completedPoints: dto.completed_points,
    incompletePoints: dto.incomplete_points,
    addedPoints: dto.added_points,
    removedPoints: dto.removed_points,
    completed: (dto.completed ?? []).map(toReportIssue),
    incomplete: (dto.incomplete ?? []).map(toReportIssue),
    removed: (dto.removed ?? []).map(toReportIssue),
    addedAfterStart: (dto.added_after_start ?? []).map(toReportIssue),
    burndown: (dto.burndown ?? []).map((point) => ({
      date: point.date,
      ideal: point.ideal ?? null,
      remaining: point.remaining,
      scope: point.scope,
      completed: point.completed,
    })),
    velocity: (dto.velocity ?? []).map((entry) => ({
      sprintId: entry.sprint_id,
      name: entry.name,
      points: entry.points,
    })),
  };
}

function toCompletion(dto: CompletionDto): SprintCompletion {
  return {
    sprint: toAgileSprint(dto.sprint),
    nextSprint: dto.next_sprint ? toAgileSprint(dto.next_sprint) : null,
    completedIssues: dto.completed_issues,
    movedIssues: dto.moved_issues,
    completedPoints: dto.completed_points,
    committedPoints: dto.committed_points,
  };
}

/** Sprint lifecycle (create, edit, start, complete, delete) and the sprint report. */
@Injectable({ providedIn: 'root' })
export class SprintV2ApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(projectId: string): Observable<AgileSprint[]> {
    return this.http
      .get<Envelope<AgileSprintDto[]>>(`${this.baseUrl}/sprints`, {
        params: { project_id: projectId },
        context: silentRequest(),
      })
      .pipe(map((body) => body.data.map(toAgileSprint)));
  }

  get(sprintId: string): Observable<AgileSprint> {
    return this.http
      .get<Envelope<AgileSprintDto>>(`${this.baseUrl}/sprints/${sprintId}`, {
        context: silentRequest(),
      })
      .pipe(map((body) => toAgileSprint(body.data)));
  }

  create(projectId: string, input: CreateAgileSprintInput): Observable<AgileSprint> {
    return this.http
      .post<Envelope<AgileSprintDto>>(
        `${this.baseUrl}/sprints`,
        {
          name: input.name,
          goal: input.goal ?? '',
          start_date: input.startDate || undefined,
          end_date: input.endDate || undefined,
        },
        { params: { project_id: projectId }, context: silentRequest() },
      )
      .pipe(map((body) => toAgileSprint(body.data)));
  }

  update(sprintId: string, input: UpdateAgileSprintInput): Observable<AgileSprint> {
    return this.http
      .patch<Envelope<AgileSprintDto>>(
        `${this.baseUrl}/sprints/${sprintId}`,
        {
          name: input.name,
          goal: input.goal,
          start_date: input.startDate,
          end_date: input.endDate,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toAgileSprint(body.data)));
  }

  delete(sprintId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/sprints/${sprintId}`, {
      context: silentRequest(),
    });
  }

  start(sprintId: string, input: StartSprintInput): Observable<AgileSprint> {
    return this.http
      .post<Envelope<AgileSprintDto>>(
        `${this.baseUrl}/sprints/${sprintId}/start`,
        {
          name: input.name,
          goal: input.goal,
          start_date: input.startDate,
          end_date: input.endDate,
          force: input.force,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toAgileSprint(body.data)));
  }

  complete(sprintId: string, input: CompleteSprintInput): Observable<SprintCompletion> {
    return this.http
      .post<Envelope<CompletionDto>>(
        `${this.baseUrl}/sprints/${sprintId}/complete`,
        { destination: input.destination, new_sprint_name: input.newSprintName },
        { context: silentRequest() },
      )
      .pipe(map((body) => toCompletion(body.data)));
  }

  report(sprintId: string): Observable<SprintReport> {
    return this.http
      .get<Envelope<ReportDto>>(`${this.baseUrl}/sprints/${sprintId}/report`, {
        context: silentRequest(),
      })
      .pipe(map((body) => toReport(body.data)));
  }
}
