import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  BoardCardField,
  BoardColumnConfig,
  BoardColumnInput,
  BoardDetail,
  BoardEstimation,
  BoardMoveInput,
  BoardMoveResult,
  BoardNamed,
  BoardQuery,
  BoardQueryInput,
  BoardRef,
  BoardSummary,
  BoardSwimlaneMode,
  BoardView,
  BoardViewCard,
  BoardViewColumn,
  BoardViewRequest,
  BoardViewSwimlane,
  BoardViewUnmapped,
  BoardWipResult,
  BoardWorkflowStatus,
  CreateBoardInput,
  StatusCategory,
  UpdateBoardInput,
  WipState,
} from '../../models/board-v2.model';
import { silentRequest } from '../../utils/silent-request';

interface Envelope<T> {
  readonly data: T;
}

interface SummaryDto {
  id: string;
  project_id: string;
  name: string;
  type: 'scrum' | 'kanban';
  is_default: boolean;
  column_count: number;
  swimlane_mode: BoardSwimlaneMode;
}

interface ColumnConfigDto {
  id: string;
  name: string;
  color?: string;
  position: number;
  min_issues?: number | null;
  max_issues?: number | null;
  status_ids?: string[] | null;
}

interface QueryDto {
  id: string;
  name: string;
  query: string;
}

interface WorkflowStatusDto {
  id: string;
  name: string;
  category: StatusCategory;
}

interface DetailDto {
  id: string;
  project_id: string;
  name: string;
  type: 'scrum' | 'kanban';
  is_default: boolean;
  swimlane_mode: BoardSwimlaneMode;
  estimation: BoardEstimation;
  card_fields?: BoardCardField[] | null;
  show_unmapped_lane: boolean;
  done_retention_days?: number | null;
  columns?: ColumnConfigDto[] | null;
  quick_filters?: QueryDto[] | null;
  swimlanes?: QueryDto[] | null;
  workflow_statuses?: WorkflowStatusDto[] | null;
  unmapped_status_ids?: string[] | null;
  can_manage_board?: boolean;
  can_move_cards?: boolean;
}

interface NamedDto {
  name: string;
  key?: string;
  icon?: string;
  color?: string;
}

interface RefDto {
  id: string;
  key: string;
  summary: string;
}

interface CardDto {
  id: string;
  key: string;
  summary: string;
  type: NamedDto;
  priority: NamedDto;
  status: WorkflowStatusDto;
  assignee?: { id: string; name: string } | null;
  labels?: string[] | null;
  story_points?: number | null;
  remaining_estimate_seconds?: number | null;
  flagged?: boolean;
  rank: string;
  sprint_id?: string | null;
  epic?: RefDto | null;
  parent?: RefDto | null;
  subtasks?: { total: number; done: number } | null;
  swimlane_id?: string;
}

interface ViewColumnDto extends ColumnConfigDto {
  count: number;
  total: number;
  estimate: number;
  wip_state: WipState;
  cards?: CardDto[] | null;
}

interface UnmappedDto {
  id: string;
  name: string;
  status_ids?: string[] | null;
  count: number;
  total: number;
  estimate: number;
  cards?: CardDto[] | null;
}

interface ViewDto {
  board: DetailDto;
  sprint?: { id: string; name: string } | null;
  columns?: ViewColumnDto[] | null;
  unmapped_column?: UnmappedDto | null;
  unmapped_count?: number;
  swimlanes?: { id: string; name: string; kind: BoardSwimlaneMode; count: number }[] | null;
}

interface MoveResultDto {
  issue_id: string;
  column_id: string;
  status: WorkflowStatusDto;
  transition_id: string;
  transitioned: boolean;
  rank?: string;
  wip?: {
    column_id: string;
    count: number;
    min_issues?: number | null;
    max_issues?: number | null;
    state: WipState;
    exceeded: boolean;
  } | null;
}

function toSummary(dto: SummaryDto): BoardSummary {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    type: dto.type,
    isDefault: dto.is_default,
    columnCount: dto.column_count,
    swimlaneMode: dto.swimlane_mode,
  };
}

function toColumnConfig(dto: ColumnConfigDto): BoardColumnConfig {
  return {
    id: dto.id,
    name: dto.name,
    color: dto.color ?? '',
    position: dto.position,
    minIssues: dto.min_issues ?? null,
    maxIssues: dto.max_issues ?? null,
    statusIds: dto.status_ids ?? [],
  };
}

function toQuery(dto: QueryDto): BoardQuery {
  return { id: dto.id, name: dto.name, query: dto.query };
}

function toStatus(dto: WorkflowStatusDto): BoardWorkflowStatus {
  return { id: dto.id, name: dto.name, category: dto.category };
}

export function toBoardDetail(dto: DetailDto): BoardDetail {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    type: dto.type,
    isDefault: dto.is_default,
    swimlaneMode: dto.swimlane_mode,
    estimation: dto.estimation,
    cardFields: dto.card_fields ?? [],
    showUnmappedLane: dto.show_unmapped_lane,
    doneRetentionDays: dto.done_retention_days ?? null,
    columns: (dto.columns ?? []).map(toColumnConfig),
    quickFilters: (dto.quick_filters ?? []).map(toQuery),
    swimlanes: (dto.swimlanes ?? []).map(toQuery),
    workflowStatuses: (dto.workflow_statuses ?? []).map(toStatus),
    unmappedStatusIds: dto.unmapped_status_ids ?? [],
    canManageBoard: dto.can_manage_board === true,
    canMoveCards: dto.can_move_cards === true,
  };
}

function toNamed(dto: NamedDto): BoardNamed {
  return { name: dto.name, key: dto.key ?? '', icon: dto.icon ?? '', color: dto.color ?? '' };
}

function toRef(dto: RefDto | null | undefined): BoardRef | null {
  return dto ? { id: dto.id, key: dto.key, summary: dto.summary } : null;
}

function toCard(dto: CardDto): BoardViewCard {
  return {
    id: dto.id,
    key: dto.key,
    summary: dto.summary,
    type: toNamed(dto.type),
    priority: toNamed(dto.priority),
    status: toStatus(dto.status),
    assignee: dto.assignee ?? null,
    labels: dto.labels ?? [],
    storyPoints: dto.story_points ?? null,
    remainingEstimateSeconds: dto.remaining_estimate_seconds ?? null,
    flagged: dto.flagged === true,
    rank: dto.rank,
    sprintId: dto.sprint_id ?? null,
    epic: toRef(dto.epic),
    parent: toRef(dto.parent),
    subtasks: dto.subtasks ?? { total: 0, done: 0 },
    swimlaneId: dto.swimlane_id ?? '',
  };
}

function toViewColumn(dto: ViewColumnDto): BoardViewColumn {
  return {
    ...toColumnConfig(dto),
    count: dto.count,
    total: dto.total,
    estimate: dto.estimate,
    wipState: dto.wip_state,
    cards: (dto.cards ?? []).map(toCard),
  };
}

function toUnmapped(dto: UnmappedDto): BoardViewUnmapped {
  return {
    id: dto.id,
    name: dto.name,
    statusIds: dto.status_ids ?? [],
    count: dto.count,
    total: dto.total,
    estimate: dto.estimate,
    cards: (dto.cards ?? []).map(toCard),
  };
}

function toSwimlane(dto: NonNullable<ViewDto['swimlanes']>[number]): BoardViewSwimlane {
  return { id: dto.id, name: dto.name, kind: dto.kind, count: dto.count };
}

function toView(dto: ViewDto): BoardView {
  return {
    board: toBoardDetail(dto.board),
    sprint: dto.sprint ?? null,
    columns: (dto.columns ?? []).map(toViewColumn),
    unmappedColumn: dto.unmapped_column ? toUnmapped(dto.unmapped_column) : null,
    unmappedCount: dto.unmapped_count ?? 0,
    swimlanes: (dto.swimlanes ?? []).map(toSwimlane),
  };
}

function toWip(dto: NonNullable<MoveResultDto['wip']>): BoardWipResult {
  return {
    columnId: dto.column_id,
    count: dto.count,
    minIssues: dto.min_issues ?? null,
    maxIssues: dto.max_issues ?? null,
    state: dto.state,
    exceeded: dto.exceeded,
  };
}

function toMoveResult(dto: MoveResultDto): BoardMoveResult {
  return {
    issueId: dto.issue_id,
    columnId: dto.column_id,
    status: toStatus(dto.status),
    transitionId: dto.transition_id,
    transitioned: dto.transitioned,
    rank: dto.rank ?? '',
    wip: dto.wip ? toWip(dto.wip) : null,
  };
}

function toColumnPayload(column: BoardColumnInput): Record<string, unknown> {
  return {
    id: column.id,
    name: column.name,
    color: column.color,
    min_issues: column.minIssues,
    max_issues: column.maxIssues,
    status_ids: column.statusIds,
  };
}

/** Boards of a project: list, settings, columns, quick filters, swimlanes, view and card moves. */
@Injectable({ providedIn: 'root' })
export class BoardV2ApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(projectId: string): Observable<BoardSummary[]> {
    return this.http
      .get<Envelope<SummaryDto[]>>(`${this.baseUrl}/projects/${projectId}/boards`, {
        context: silentRequest(),
      })
      .pipe(map((body) => body.data.map(toSummary)));
  }

  get(boardId: string): Observable<BoardDetail> {
    return this.http
      .get<Envelope<DetailDto>>(`${this.baseUrl}/boards/${boardId}`, { context: silentRequest() })
      .pipe(map((body) => toBoardDetail(body.data)));
  }

  create(projectId: string, input: CreateBoardInput): Observable<BoardDetail> {
    return this.http
      .post<Envelope<DetailDto>>(
        `${this.baseUrl}/projects/${projectId}/boards`,
        { name: input.name, type: input.type },
        { context: silentRequest() },
      )
      .pipe(map((body) => toBoardDetail(body.data)));
  }

  update(boardId: string, input: UpdateBoardInput): Observable<BoardDetail> {
    return this.http
      .patch<Envelope<DetailDto>>(
        `${this.baseUrl}/boards/${boardId}`,
        {
          name: input.name,
          swimlane_mode: input.swimlaneMode,
          estimation: input.estimation,
          card_fields: input.cardFields,
          show_unmapped_lane: input.showUnmappedLane,
          done_retention_days: input.doneRetentionDays,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toBoardDetail(body.data)));
  }

  delete(boardId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/boards/${boardId}`, {
      context: silentRequest(),
    });
  }

  replaceColumns(boardId: string, columns: readonly BoardColumnInput[]): Observable<BoardDetail> {
    return this.http
      .put<Envelope<DetailDto>>(
        `${this.baseUrl}/boards/${boardId}/columns`,
        { columns: columns.map(toColumnPayload) },
        { context: silentRequest() },
      )
      .pipe(map((body) => toBoardDetail(body.data)));
  }

  replaceQuickFilters(
    boardId: string,
    quickFilters: readonly BoardQueryInput[],
  ): Observable<BoardDetail> {
    return this.http
      .put<Envelope<DetailDto>>(
        `${this.baseUrl}/boards/${boardId}/quick-filters`,
        { quick_filters: quickFilters },
        { context: silentRequest() },
      )
      .pipe(map((body) => toBoardDetail(body.data)));
  }

  replaceSwimlanes(
    boardId: string,
    swimlanes: readonly BoardQueryInput[],
  ): Observable<BoardDetail> {
    return this.http
      .put<Envelope<DetailDto>>(
        `${this.baseUrl}/boards/${boardId}/swimlanes`,
        { swimlanes },
        { context: silentRequest() },
      )
      .pipe(map((body) => toBoardDetail(body.data)));
  }

  view(boardId: string, request: BoardViewRequest): Observable<BoardView> {
    let params = new HttpParams();
    if (request.quickFilterIds.length > 0) {
      params = params.set('quick_filters', request.quickFilterIds.join(','));
    }
    if (request.sprint) {
      params = params.set('sprint', request.sprint);
    }
    return this.http
      .get<Envelope<ViewDto>>(`${this.baseUrl}/boards/${boardId}/view`, {
        params,
        context: silentRequest(),
      })
      .pipe(map((body) => toView(body.data)));
  }

  /** Moves a card to a column. Failures carry the codes of the move (ambiguous, unavailable, required fields). */
  moveCard(boardId: string, issueId: string, input: BoardMoveInput): Observable<BoardMoveResult> {
    return this.http
      .post<Envelope<MoveResultDto>>(
        `${this.baseUrl}/boards/${boardId}/cards/${issueId}/move`,
        {
          column_id: input.columnId,
          before_issue_id: input.beforeIssueId,
          after_issue_id: input.afterIssueId,
          swimlane: input.swimlane,
          transition_id: input.transitionId,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toMoveResult(body.data)));
  }
}
