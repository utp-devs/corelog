import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  BacklogCategoryTotals,
  BacklogEpic,
  BacklogIssue,
  BacklogQuery,
  BacklogSection,
  BacklogSectionKind,
  BacklogTotals,
  BacklogView,
  RankBacklogInput,
} from '../../models/backlog-v2.model';
import type { StatusCategory } from '../../models/board-v2.model';
import { silentRequest } from '../../utils/silent-request';
import { AgileSprintDto, toAgileSprint } from './sprint-v2-api.service';

interface Envelope<T> {
  readonly data: T;
}

interface CategoryTotalsDto {
  issues: number;
  points: number;
  original_estimate_seconds: number;
}

interface IssueDto {
  id: string;
  key: string;
  summary: string;
  issue_type: string;
  issue_type_name?: string;
  issue_type_icon?: string;
  issue_type_color?: string;
  status: string;
  status_name?: string;
  status_category: StatusCategory;
  priority: string;
  priority_name?: string;
  priority_icon?: string;
  priority_color?: string;
  assignee_id?: string | null;
  assignee_name?: string;
  story_points?: number | null;
  original_estimate_seconds?: number | null;
  rank: string;
  epic_id?: string | null;
  epic_key?: string;
  epic_summary?: string;
  flagged?: boolean;
  labels?: { id: string; name: string; color: string }[] | null;
}

interface SectionDto {
  kind: BacklogSectionKind;
  sprint?: AgileSprintDto | null;
  issues?: IssueDto[] | null;
  totals?:
    | (CategoryTotalsDto & { by_category?: Partial<Record<StatusCategory, CategoryTotalsDto>> })
    | null;
}

interface EpicDto {
  id: string;
  key: string;
  summary: string;
  status: StatusCategory;
  total: number;
  done: number;
  points: number;
  done_points: number;
  progress: number;
}

interface BacklogDto {
  project_id: string;
  sections?: SectionDto[] | null;
  epics?: EpicDto[] | null;
  can_manage_sprints?: boolean;
  can_move_cards?: boolean;
  truncated?: boolean;
}

function toCategoryTotals(dto: CategoryTotalsDto | undefined | null): BacklogCategoryTotals {
  return {
    issues: dto?.issues ?? 0,
    points: dto?.points ?? 0,
    originalEstimateSeconds: dto?.original_estimate_seconds ?? 0,
  };
}

function toTotals(dto: SectionDto['totals']): BacklogTotals {
  const byCategory: Partial<Record<StatusCategory, BacklogCategoryTotals>> = {};
  for (const category of ['todo', 'in_progress', 'done'] as const) {
    byCategory[category] = toCategoryTotals(dto?.by_category?.[category]);
  }
  return { ...toCategoryTotals(dto), byCategory };
}

function toIssue(dto: IssueDto): BacklogIssue {
  return {
    id: dto.id,
    key: dto.key,
    summary: dto.summary,
    issueType: dto.issue_type,
    issueTypeName: dto.issue_type_name ?? dto.issue_type,
    issueTypeIcon: dto.issue_type_icon ?? dto.issue_type,
    issueTypeColor: dto.issue_type_color ?? '',
    status: dto.status,
    statusName: dto.status_name ?? dto.status,
    statusCategory: dto.status_category,
    priority: dto.priority,
    priorityName: dto.priority_name ?? dto.priority,
    priorityIcon: dto.priority_icon ?? '',
    priorityColor: dto.priority_color ?? '',
    assigneeId: dto.assignee_id ?? null,
    assigneeName: dto.assignee_name ?? '',
    storyPoints: dto.story_points ?? null,
    originalEstimateSeconds: dto.original_estimate_seconds ?? null,
    rank: dto.rank,
    epicId: dto.epic_id ?? null,
    epicKey: dto.epic_key ?? '',
    epicSummary: dto.epic_summary ?? '',
    flagged: dto.flagged === true,
    labels: dto.labels ?? [],
  };
}

function toSection(dto: SectionDto): BacklogSection {
  return {
    kind: dto.kind,
    sprint: dto.sprint ? toAgileSprint(dto.sprint) : null,
    issues: (dto.issues ?? []).map(toIssue),
    totals: toTotals(dto.totals),
  };
}

function toEpic(dto: EpicDto): BacklogEpic {
  return {
    id: dto.id,
    key: dto.key,
    summary: dto.summary,
    status: dto.status,
    total: dto.total,
    done: dto.done,
    points: dto.points,
    donePoints: dto.done_points,
    progress: dto.progress,
  };
}

/** Backlog of a project (sprint sections plus the unscheduled backlog) and its ranking. */
@Injectable({ providedIn: 'root' })
export class BacklogV2ApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  get(projectId: string, query: BacklogQuery = {}): Observable<BacklogView> {
    let params = new HttpParams();
    if (query.epic) params = params.set('epic', query.epic);
    if (query.q) params = params.set('q', query.q);
    return this.http
      .get<Envelope<BacklogDto>>(`${this.baseUrl}/projects/${projectId}/backlog`, {
        params,
        context: silentRequest(),
      })
      .pipe(
        map(({ data }) => ({
          projectId: data.project_id,
          sections: (data.sections ?? []).map(toSection),
          epics: (data.epics ?? []).map(toEpic),
          canManageSprints: data.can_manage_sprints === true,
          canMoveCards: data.can_move_cards === true,
          truncated: data.truncated === true,
        })),
      );
  }

  /** Moves issues to a section and places them next to an anchor; without anchor they go to the end. */
  rank(input: RankBacklogInput): Observable<void> {
    return this.http
      .put<void>(
        `${this.baseUrl}/backlog/rank`,
        {
          issue_ids: input.issueIds,
          target: { sprint_id: input.sprintId },
          before_issue_id: input.beforeIssueId,
          after_issue_id: input.afterIssueId,
        },
        { context: silentRequest() },
      )
      .pipe(map(() => undefined));
  }

  /** Sets or clears the story points of an issue. */
  setPoints(issueId: string, points: number | null): Observable<void> {
    return this.http
      .patch<void>(
        `${this.baseUrl}/issues/${issueId}`,
        { story_points: points },
        { context: silentRequest() },
      )
      .pipe(map(() => undefined));
  }
}
