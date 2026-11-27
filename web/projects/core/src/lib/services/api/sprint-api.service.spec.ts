import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { SprintApiService } from './sprint-api.service';

const sprintDto = {
  id: 'sprint-1',
  project_id: 'project-1',
  name: 'Sprint 1',
  goal: 'Entregar exportacion',
  status: 'active',
  start_date: '2026-11-02',
  end_date: '2026-11-08',
  closed_at: null,
};

function setup(): { service: SprintApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(SprintApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('SprintApiService', () => {
  it('maps sprints from snake_case and drops null dates', () => {
    const { service, backend } = setup();
    let sprints: {
      id: string;
      projectId: string;
      status: string;
      startDate?: string;
      endDate?: string;
      closedAt?: string;
    }[] = [];

    service.list('project-1').subscribe((value) => (sprints = value));
    backend.expectOne('/api/v1/sprints?project_id=project-1').flush({ data: [sprintDto] });

    expect(sprints[0]).toMatchObject({
      id: 'sprint-1',
      projectId: 'project-1',
      status: 'active',
      startDate: '2026-11-02',
      endDate: '2026-11-08',
    });
    expect(sprints[0].closedAt).toBeUndefined();
    backend.verify();
  });

  it('sends the close option and the target sprint', () => {
    const { service, backend } = setup();

    service.close('project-1', 'sprint-1', 'sprint', 'sprint-2').subscribe();
    const request = backend.expectOne('/api/v1/sprints/sprint-1/close?project_id=project-1');
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({ unfinished: 'sprint', target_sprint_id: 'sprint-2' });
    request.flush({ data: { ...sprintDto, status: 'closed' } });
    backend.verify();
  });

  it('sends null target and null dates when they are omitted', () => {
    const { service, backend } = setup();

    service.close('project-1', 'sprint-1', 'backlog').subscribe();
    const close = backend.expectOne('/api/v1/sprints/sprint-1/close?project_id=project-1');
    expect(close.request.body).toEqual({ unfinished: 'backlog', target_sprint_id: null });
    close.flush({ data: sprintDto });

    service.create('project-1', { name: 'Sprint 1', goal: '' }).subscribe();
    const create = backend.expectOne('/api/v1/sprints?project_id=project-1');
    expect(create.request.body).toEqual({
      name: 'Sprint 1',
      goal: '',
      start_date: null,
      end_date: null,
    });
    create.flush({ data: sprintDto });
    backend.verify();
  });

  it('maps velocity points and requests the default limit of 5', () => {
    const { service, backend } = setup();
    let points: { sprintId: string; name: string; points: number }[] = [];

    service.velocity('project-1').subscribe((value) => (points = value));
    backend
      .expectOne('/api/v1/sprints/velocity?project_id=project-1&limit=5')
      .flush({ data: [{ sprint_id: 'sprint-0', name: 'Sprint 0', points: 13 }] });

    expect(points).toEqual([{ sprintId: 'sprint-0', name: 'Sprint 0', points: 13 }]);
    backend.verify();
  });

  it('maps issues, burndown and the start, assign and remove calls', () => {
    const { service, backend } = setup();
    let issues: { id: string; storyPoints?: number; closedAt?: string }[] = [];

    service.listIssues('project-1', 'sprint-1').subscribe((value) => (issues = value));
    backend.expectOne('/api/v1/sprints/sprint-1/issues?project_id=project-1').flush({
      data: [
        {
          id: 'issue-1',
          title: 'Exportar',
          status: 'open',
          priority: 'low',
          story_points: null,
          closed_at: null,
        },
      ],
    });
    expect(issues).toEqual([
      {
        id: 'issue-1',
        title: 'Exportar',
        status: 'open',
        priority: 'low',
        storyPoints: undefined,
        closedAt: undefined,
      },
    ]);

    service.start('project-1', 'sprint-1').subscribe();
    const start = backend.expectOne('/api/v1/sprints/sprint-1/start?project_id=project-1');
    expect(start.request.method).toBe('POST');
    start.flush({ data: sprintDto });

    service.assignIssue('project-1', 'sprint-1', 'issue-1').subscribe();
    const assign = backend.expectOne(
      '/api/v1/sprints/sprint-1/issues/issue-1?project_id=project-1',
    );
    expect(assign.request.method).toBe('PUT');
    assign.flush(null, { status: 204, statusText: 'No Content' });

    service.removeIssue('project-1', 'sprint-1', 'issue-1').subscribe();
    const remove = backend.expectOne(
      '/api/v1/sprints/sprint-1/issues/issue-1?project_id=project-1',
    );
    expect(remove.request.method).toBe('DELETE');
    remove.flush(null, { status: 204, statusText: 'No Content' });

    service.burndown('project-1', 'sprint-1').subscribe();
    backend
      .expectOne('/api/v1/sprints/sprint-1/burndown?project_id=project-1')
      .flush({ data: [{ date: '2026-11-02', ideal: 10, remaining: 10 }] });
    backend.verify();
  });
});
