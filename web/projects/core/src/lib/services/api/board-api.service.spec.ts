import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { ProjectBoard } from '../../models/board.model';
import { BoardApiService } from './board-api.service';

const boardDto = {
  id: 'board-1',
  project_id: 'project-1',
  name: 'Default',
  type: 'kanban',
  can_manage_board: true,
  columns: [
    {
      id: 'column-1',
      name: 'En curso',
      status: 'in_progress',
      position: 1,
      wip_limit: 5,
      count: 1,
      cards: [
        {
          id: 'issue-1',
          type: 'bug',
          summary: 'Error al exportar',
          status: 'in_progress',
          priority: 'high',
          assignee_id: 'user-1',
          assignee: 'Ada Lovelace',
          story_points: 3,
          rank: 'm',
        },
      ],
    },
  ],
};

function setup(): { service: BoardApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(BoardApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('BoardApiService', () => {
  it('maps the board from snake_case to the domain shape', () => {
    const { service, backend } = setup();
    let board: ProjectBoard | undefined;

    service.getProjectBoard('project-1').subscribe((value) => (board = value));
    backend.expectOne('/api/v1/projects/project-1/board').flush({ data: boardDto });

    expect(board?.projectId).toBe('project-1');
    expect(board?.canManageBoard).toBe(true);
    const [column] = board?.columns ?? [];
    expect(column.wipLimit).toBe(5);
    expect(column.cards[0].storyPoints).toBe(3);
    expect(column.cards[0].assigneeId).toBe('user-1');
    backend.verify();
  });

  it('maps null optional fields to undefined', () => {
    const { service, backend } = setup();
    let board: ProjectBoard | undefined;
    const [column] = boardDto.columns;
    const [card] = column.cards;
    const nullable = {
      ...boardDto,
      columns: [
        {
          ...column,
          wip_limit: null,
          cards: [{ ...card, assignee_id: null, story_points: null }],
        },
      ],
    };

    service.getProjectBoard('project-1').subscribe((value) => (board = value));
    backend.expectOne('/api/v1/projects/project-1/board').flush({ data: nullable });

    expect(board?.columns[0].wipLimit).toBeUndefined();
    expect(board?.columns[0].cards[0].assigneeId).toBeUndefined();
    expect(board?.columns[0].cards[0].storyPoints).toBeUndefined();
    backend.verify();
  });

  it('sends the move request with column_id and returns the updated board', () => {
    const { service, backend } = setup();
    let board: ProjectBoard | undefined;

    service.moveCard('project-1', 'issue-1', 'column-1').subscribe((value) => (board = value));
    const request = backend.expectOne('/api/v1/projects/project-1/board/cards/issue-1/move');
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({ column_id: 'column-1' });
    request.flush({ data: boardDto });

    expect(board?.columns[0].cards).toHaveLength(1);
    backend.verify();
  });

  it('sends null to clear a WIP limit', () => {
    const { service, backend } = setup();

    service.updateColumnWip('project-1', 'column-1', null).subscribe();
    const request = backend.expectOne('/api/v1/projects/project-1/board/columns/column-1');
    expect(request.request.method).toBe('PATCH');
    expect(request.request.body).toEqual({ wip_limit: null });
    request.flush({ data: boardDto });
    backend.verify();
  });
});
