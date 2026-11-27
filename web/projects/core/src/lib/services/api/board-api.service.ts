import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  BoardCard,
  BoardColumn,
  BoardType,
  ProjectBoard,
  WorkItemPriority,
  WorkItemStatus,
} from '../../models/board.model';
import { silentRequest } from '../../utils/silent-request';

interface BoardCardDto {
  readonly id: string;
  readonly type: string;
  readonly summary: string;
  readonly status: WorkItemStatus;
  readonly priority: WorkItemPriority;
  readonly assignee_id?: string | null;
  readonly assignee?: string | null;
  readonly story_points?: number | null;
  readonly rank: string;
}

interface BoardColumnDto {
  readonly id: string;
  readonly name: string;
  readonly status: WorkItemStatus;
  readonly position: number;
  readonly wip_limit?: number | null;
  readonly count: number;
  readonly cards: readonly BoardCardDto[];
}

interface BoardDto {
  readonly id: string;
  readonly project_id: string;
  readonly name: string;
  readonly type: BoardType;
  readonly columns: readonly BoardColumnDto[];
  readonly can_manage_board?: boolean;
}

interface DataEnvelope<T> {
  readonly data: T;
}

function toCard(card: BoardCardDto): BoardCard {
  return {
    id: card.id,
    type: card.type,
    summary: card.summary,
    status: card.status,
    priority: card.priority,
    assigneeId: card.assignee_id ?? undefined,
    assignee: card.assignee ?? undefined,
    storyPoints: card.story_points ?? undefined,
    rank: card.rank,
  };
}

function toColumn(column: BoardColumnDto): BoardColumn {
  return {
    id: column.id,
    name: column.name,
    status: column.status,
    position: column.position,
    wipLimit: column.wip_limit ?? undefined,
    count: column.count,
    cards: column.cards.map(toCard),
  };
}

function toBoard(board: BoardDto): ProjectBoard {
  return {
    id: board.id,
    projectId: board.project_id,
    name: board.name,
    type: board.type,
    columns: board.columns.map(toColumn),
    canManageBoard: board.can_manage_board === true,
  };
}

/** Default board of a project, its columns and cards. */
@Injectable({ providedIn: 'root' })
export class BoardApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  getProjectBoard(projectId: string): Observable<ProjectBoard> {
    return this.http
      .get<DataEnvelope<BoardDto>>(`${this.baseUrl}/projects/${projectId}/board`, {
        context: silentRequest(),
      })
      .pipe(map((response) => toBoard(response.data)));
  }

  /** Moves an issue to another column. The column defines the new status. Returns the updated board. */
  moveCard(projectId: string, issueId: string, columnId: string): Observable<ProjectBoard> {
    return this.http
      .post<DataEnvelope<BoardDto>>(
        `${this.baseUrl}/projects/${projectId}/board/cards/${issueId}/move`,
        { column_id: columnId },
        { context: silentRequest() },
      )
      .pipe(map((response) => toBoard(response.data)));
  }

  /** Sets the WIP limit of a column. `null` removes it. Returns the updated board. */
  updateColumnWip(
    projectId: string,
    columnId: string,
    wipLimit: number | null,
  ): Observable<ProjectBoard> {
    return this.http
      .patch<DataEnvelope<BoardDto>>(
        `${this.baseUrl}/projects/${projectId}/board/columns/${columnId}`,
        { wip_limit: wipLimit },
        { context: silentRequest() },
      )
      .pipe(map((response) => toBoard(response.data)));
  }
}
