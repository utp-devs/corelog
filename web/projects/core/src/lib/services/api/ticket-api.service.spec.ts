import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';

import { provideCorelogTesting } from '../../../testing';
import { TicketPage } from '../../models/ticket.model';
import { asApiError } from '../../utils/api-error';
import { TicketApiService } from './ticket-api.service';

const ISSUE_ID = '11111111-1111-4111-8111-111111111111';
const ASSIGNEE_ID = '33333333-3333-4333-8333-333333333333';

const issue = {
  id: ISSUE_ID,
  project_id: '00000000-0000-4000-8000-000000000001',
  title: 'Error al exportar',
  description: 'Detalle',
  status: 'open',
  priority: 'high',
  issue_type: 'bug',
  parent_id: null,
  due_date: null,
  story_points: null,
  rank: 'm',
  closed_at: null,
  requester_id: '22222222-2222-4222-8222-222222222222',
  requester_name: 'Ana Torres',
  assignee_id: null,
  created_at: '2026-11-22T00:00:00Z',
  updated_at: '2026-11-22T00:00:00Z',
};

describe('TicketApiService', () => {
  let service: TicketApiService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(TicketApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  describe('list', () => {
    it('maps the page items and the pagination meta to camelCase', () => {
      let result: TicketPage | undefined;
      service.list({ page: 2, pageSize: 20 }).subscribe((page) => (result = page));

      httpMock
        .expectOne((request) => request.url === '/api/v1/issues')
        .flush({ data: [issue], meta: { page: 2, page_size: 20, total: 41 } });

      const page = result;
      expect(page?.total).toBe(41);
      expect(page?.page).toBe(2);
      expect(page?.pageSize).toBe(20);
      expect(page?.items).toHaveLength(1);
      expect(page?.items[0]).toMatchObject({
        id: ISSUE_ID,
        title: 'Error al exportar',
        status: 'open',
      });
    });

    it('sends the pagination parameters to the issues endpoint', () => {
      service.list({ page: 3, pageSize: 10 }).subscribe();

      const request = httpMock.expectOne((req) => req.url === '/api/v1/issues');
      expect(request.request.params.get('page')).toBe('3');
      expect(request.request.params.get('page_size')).toBe('10');
      expect(request.request.params.has('status')).toBe(false);
      request.flush({ data: [], meta: { page: 3, page_size: 10, total: 0 } });
    });

    it('maps the requester and assignee display names', () => {
      let names: { requesterName?: string; assigneeName?: string | null } | undefined;
      service.list({ page: 1, pageSize: 20 }).subscribe((page) => (names = page.items[0]));

      httpMock
        .expectOne((req) => req.url === '/api/v1/issues')
        .flush({
          data: [{ ...issue, assignee_id: ASSIGNEE_ID, assignee_name: 'Luis Paredes' }],
          meta: { page: 1, page_size: 20, total: 1 },
        });

      expect(names?.requesterName).toBe('Ana Torres');
      expect(names?.assigneeName).toBe('Luis Paredes');
    });

    it('leaves the assignee name null when the issue is unassigned', () => {
      let assigneeName: string | null | undefined;
      service
        .list({ page: 1, pageSize: 20 })
        .subscribe((page) => (assigneeName = page.items[0]?.assigneeName));

      httpMock
        .expectOne((req) => req.url === '/api/v1/issues')
        .flush({
          data: [issue],
          meta: { page: 1, page_size: 20, total: 1 },
        });

      expect(assigneeName).toBeNull();
    });
  });

  describe('assign', () => {
    it('sends the assignee id to the assign endpoint', () => {
      let assigneeId: string | null | undefined;
      service.assign(ISSUE_ID, ASSIGNEE_ID).subscribe((ticket) => (assigneeId = ticket.assigneeId));

      const request = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/assign`);
      expect(request.request.method).toBe('PATCH');
      expect(request.request.body).toEqual({ assignee_id: ASSIGNEE_ID });
      request.flush({ data: { ...issue, assignee_id: ASSIGNEE_ID } });

      expect(assigneeId).toBe(ASSIGNEE_ID);
    });

    it('sends null to clear the assignee and maps the unassigned issue', () => {
      let result: { assigneeId: string | null; assigneeName: string | null } | undefined;
      service.assign(ISSUE_ID, null).subscribe((ticket) => (result = ticket));

      const request = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/assign`);
      expect(request.request.body).toEqual({ assignee_id: null });
      request.flush({ data: issue });

      expect(result?.assigneeId).toBeNull();
      expect(result?.assigneeName).toBeNull();
    });
  });

  describe('status and rank', () => {
    it('patches the status of the issue', () => {
      service.updateStatus(ISSUE_ID, 'in_progress').subscribe();

      const request = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/status`);
      expect(request.request.body).toEqual({ status: 'in_progress' });
      request.flush({ data: issue });
    });

    it('patches the rank of the issue', () => {
      service.updateRank(ISSUE_ID, 'n').subscribe();

      const request = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/rank`);
      expect(request.request.body).toEqual({ rank: 'n' });
      request.flush({ data: issue });
    });
  });

  describe('details, metadata and create', () => {
    it('maps the camelCase details to the snake_case body', () => {
      service
        .updateDetails(ISSUE_ID, {
          title: 'Nuevo',
          description: 'Texto',
          priority: 'low',
          issueType: 'story',
          storyPoints: 3,
        })
        .subscribe();

      const request = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/details`);
      expect(request.request.body).toEqual({
        title: 'Nuevo',
        description: 'Texto',
        priority: 'low',
        issue_type: 'story',
        parent_id: undefined,
        due_date: undefined,
        story_points: 3,
      });
      request.flush({ data: issue });
    });

    it('maps the metadata body and the created issue', () => {
      service.updateMetadata(ISSUE_ID, { issueType: 'epic', dueDate: '2026-12-01' }).subscribe();
      const metadata = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/metadata`);
      expect(metadata.request.body).toMatchObject({ issue_type: 'epic', due_date: '2026-12-01' });
      metadata.flush({ data: issue });

      let created: { title: string } | undefined;
      service
        .create({ title: 'Nueva', description: '', priority: 'medium', issueType: 'task' })
        .subscribe((ticket) => (created = ticket));
      const post = httpMock.expectOne('/api/v1/issues');
      expect(post.request.body).toMatchObject({ title: 'Nueva', issue_type: 'task' });
      post.flush({ data: { ...issue, title: 'Nueva' } });
      expect(created?.title).toBe('Nueva');
    });
  });

  describe('comments, activities, links', () => {
    it('lists, creates and deletes comments', () => {
      let comments: { body: string; deletedAt: string | null }[] = [];
      service.listComments(ISSUE_ID).subscribe((items) => (comments = items));
      httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/comments`).flush({
        data: [
          {
            id: 'c-1',
            issue_id: ISSUE_ID,
            author_id: 'u-1',
            body: 'Hola',
            created_at: 'a',
            updated_at: 'b',
            deleted_at: null,
          },
        ],
      });
      expect(comments).toEqual([expect.objectContaining({ body: 'Hola', deletedAt: null })]);

      service.createComment(ISSUE_ID, 'Nuevo').subscribe();
      const post = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/comments`);
      expect(post.request.body).toEqual({ body: 'Nuevo' });
      post.flush({
        data: {
          id: 'c-2',
          issue_id: ISSUE_ID,
          author_id: 'u-1',
          body: 'Nuevo',
          created_at: 'a',
          updated_at: 'b',
          deleted_at: null,
        },
      });

      service.deleteComment(ISSUE_ID, 'c-2').subscribe();
      httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/comments/c-2`).flush(null);
    });

    it('maps activities, including assignee changes', () => {
      let action: string | undefined;
      service.listActivities(ISSUE_ID).subscribe((items) => (action = items[0]?.action));
      httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/activities`).flush({
        data: [
          {
            id: 'a-1',
            issue_id: ISSUE_ID,
            actor_id: 'u-1',
            action: 'assignee_changed',
            field_name: 'assignee_id',
            old_value: null,
            new_value: 'u-2',
            created_at: 'x',
          },
        ],
      });
      expect(action).toBe('assignee_changed');
    });

    it('creates and deletes links with the target and kind', () => {
      let kind: string | undefined;
      service.createLink(ISSUE_ID, 'target-1', 'blocks').subscribe((link) => (kind = link.kind));
      const post = httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/links`);
      expect(post.request.body).toEqual({ target_issue_id: 'target-1', kind: 'blocks' });
      post.flush({
        data: {
          id: 'l-1',
          source_issue_id: ISSUE_ID,
          target_issue_id: 'target-1',
          kind: 'blocks',
          created_by: 'u-1',
          created_at: 'x',
        },
      });
      expect(kind).toBe('blocks');

      service.listLinks(ISSUE_ID).subscribe();
      httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/links`).flush({ data: [] });

      service.deleteLink(ISSUE_ID, 'l-1').subscribe();
      httpMock.expectOne(`/api/v1/issues/${ISSUE_ID}/links/l-1`).flush(null);
    });
  });

  describe('errors', () => {
    it('passes the API error code and field details through the caller', () => {
      let code: string | undefined;
      service
        .updateStatus(ISSUE_ID, 'closed')
        .subscribe({ error: (error: unknown) => (code = asApiError(error).code) });

      httpMock
        .expectOne(`/api/v1/issues/${ISSUE_ID}/status`)
        .flush(
          { error: { message: 'Transición no válida', code: 'issue.invalid_transition' } },
          { status: 409, statusText: 'Conflict' },
        );

      expect(code).toBe('issue.invalid_transition');
    });
  });
});
