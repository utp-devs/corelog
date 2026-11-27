import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { BacklogItem } from '../../models/backlog.model';
import { BacklogApiService } from './backlog-api.service';

const itemDto = {
  id: 'issue-1',
  project_id: 'project-1',
  summary: 'Revisar exportacion',
  type: 'bug',
  status: 'open',
  priority: 'high',
  rank: '000001',
  assignee_id: 'user-1',
  story_points: 2,
};

function setup(): { service: BacklogApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(BacklogApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('BacklogApiService', () => {
  it('lists the backlog for a project and maps the items', () => {
    const { service, backend } = setup();
    let items: BacklogItem[] = [];

    service.list('project-1').subscribe((value) => (items = value));
    backend.expectOne('/api/v1/backlog?project_id=project-1').flush({ data: [itemDto] });

    expect(items[0]).toMatchObject({
      id: 'issue-1',
      projectId: 'project-1',
      summary: 'Revisar exportacion',
      storyPoints: 2,
      assigneeId: 'user-1',
    });
    backend.verify();
  });

  it('sends the target position and returns the reordered backlog', () => {
    const { service, backend } = setup();
    let items: BacklogItem[] = [];

    service.move('project-1', 'issue-1', 0).subscribe((value) => (items = value));
    const request = backend.expectOne('/api/v1/backlog/issue-1?project_id=project-1');
    expect(request.request.method).toBe('PUT');
    expect(request.request.body).toEqual({ position: 0 });
    request.flush({ data: [itemDto] });

    expect(items).toHaveLength(1);
    backend.verify();
  });
});
