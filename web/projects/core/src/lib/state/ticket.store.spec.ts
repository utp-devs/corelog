import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Observable, Subject, of, throwError } from 'rxjs';

import { provideCorelogTesting } from '../../testing';
import { Ticket } from '../models/ticket.model';
import {
  LOADING_MEMBER_NAME,
  MEMBER_NAMES_SOURCE,
  MemberNameEntry,
  TicketStore,
  UNKNOWN_MEMBER_NAME,
} from './ticket.store';

const BASE = '/api/v1/issues';

function ticket(overrides: Partial<Ticket> = {}): Ticket {
  return {
    id: 'issue-1',
    title: 'Error',
    description: '',
    status: 'open',
    priority: 'high',
    projectId: 'project-1',
    issueType: 'bug',
    parentId: null,
    dueDate: null,
    storyPoints: null,
    rank: 'm',
    closedAt: null,
    requesterId: 'user-1',
    requesterName: 'Ana',
    assigneeId: 'user-2',
    assigneeName: 'Luis',
    createdAt: '2026-11-22T00:00:00Z',
    updatedAt: '2026-11-22T00:00:00Z',
    ...overrides,
  };
}

/** Wire format of an issue, as the API returns it. */
function issueDto(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'issue-1',
    project_id: 'project-1',
    title: 'Error',
    description: '',
    status: 'open',
    priority: 'high',
    issue_type: 'bug',
    parent_id: null,
    due_date: null,
    story_points: null,
    rank: 'm',
    closed_at: null,
    requester_id: 'user-1',
    requester_name: 'Ana',
    assignee_id: 'user-2',
    assignee_name: 'Luis',
    created_at: '2026-11-22T00:00:00Z',
    updated_at: '2026-11-22T00:00:00Z',
    ...overrides,
  };
}

function comment(id: string, issueId: string) {
  return {
    id,
    issue_id: issueId,
    author_id: 'user-1',
    body: id,
    created_at: 'a',
    updated_at: 'a',
    deleted_at: null,
  };
}

describe('TicketStore', () => {
  let store: TicketStore;
  let httpMock: HttpTestingController;
  let listMembers: (organizationId: string) => Observable<readonly MemberNameEntry[]>;
  let membersCalls: string[];

  beforeEach(() => {
    membersCalls = [];
    listMembers = () => of([]);
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideHttpClient(),
        provideHttpClientTesting(),
        {
          provide: MEMBER_NAMES_SOURCE,
          useValue: {
            listMembers: (organizationId: string) => {
              membersCalls.push(organizationId);
              return listMembers(organizationId);
            },
          },
        },
      ],
    });
    store = TestBed.inject(TicketStore);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  describe('issue-scoped collaboration', () => {
    it('ignores an older comment response after switching issues', () => {
      store.fetchComments('issue-old');
      store.fetchComments('issue-new');

      httpMock.expectOne(`${BASE}/issue-old/comments`);
      httpMock
        .expectOne(`${BASE}/issue-new/comments`)
        .flush({ data: [comment('new-comment', 'issue-new')] });

      expect(store.commentsIssueId()).toBe('issue-new');
      expect(store.comments.value()?.map((item) => item.id)).toEqual(['new-comment']);
    });

    it('does not replace the selected issue with a slower previous request', () => {
      store.fetchTicket('issue-old');
      store.fetchTicket('issue-new');

      httpMock.expectOne(`${BASE}/issue-old`);
      httpMock
        .expectOne(`${BASE}/issue-new`)
        .flush({ data: issueDto({ id: 'issue-new', title: 'new' }) });

      expect(store.selected.value()?.id).toBe('issue-new');
      expect(store.selected.loading()).toBe(false);
      expect(store.selected.error()).toBeNull();
    });

    it('ignores a comment created for an issue that is no longer shown', () => {
      store.fetchComments('issue-1');
      httpMock.expectOne(`${BASE}/issue-1/comments`).flush({ data: [comment('c-1', 'issue-1')] });

      store.createComment('issue-2', 'Otro').subscribe();
      httpMock.expectOne(`${BASE}/issue-2/comments`).flush({ data: comment('c-2', 'issue-2') });

      expect(store.comments.value()?.map((item) => item.id)).toEqual(['c-1']);
    });

    it('appends a created comment and removes a deleted one for the shown issue', () => {
      store.fetchComments('issue-1');
      httpMock.expectOne(`${BASE}/issue-1/comments`).flush({ data: [comment('c-1', 'issue-1')] });

      store.createComment('issue-1', 'Nuevo').subscribe();
      httpMock.expectOne(`${BASE}/issue-1/comments`).flush({ data: comment('c-2', 'issue-1') });
      expect(store.comments.value()?.map((item) => item.id)).toEqual(['c-1', 'c-2']);

      store.deleteComment('issue-1', 'c-1').subscribe();
      httpMock.expectOne(`${BASE}/issue-1/comments/c-1`).flush(null);
      expect(store.comments.value()?.map((item) => item.id)).toEqual(['c-2']);
    });

    it('keeps watchers of the shown issue only', () => {
      store.fetchWatchers('issue-1');
      httpMock
        .expectOne(`${BASE}/issue-1/watchers`)
        .flush({ data: { is_watching: false, count: 1 } });
      expect(store.watchers.value()).toEqual({ isWatching: false, count: 1 });

      store.watch('issue-1').subscribe();
      httpMock
        .expectOne(`${BASE}/issue-1/watchers`)
        .flush({ data: { is_watching: true, count: 2 } });
      expect(store.watchers.value()).toEqual({ isWatching: true, count: 2 });

      store.unwatch('issue-2').subscribe();
      httpMock
        .expectOne(`${BASE}/issue-2/watchers`)
        .flush({ data: { is_watching: false, count: 0 } });
      expect(store.watchers.value()).toEqual({ isWatching: true, count: 2 });
    });

    it('appends an uploaded attachment only for the shown issue', () => {
      store.fetchAttachments('issue-1');
      httpMock.expectOne(`${BASE}/issue-1/attachments`).flush({ data: [] });

      const file = new File(['x'], 'nota.txt', { type: 'text/plain' });
      store.uploadAttachment('issue-1', file).subscribe();
      httpMock.expectOne(`${BASE}/issue-1/attachments`).flush({
        data: {
          id: 'att-1',
          issue_id: 'issue-1',
          uploader_id: 'user-1',
          file_name: 'nota.txt',
          content_type: 'text/plain',
          size_bytes: 1,
          created_at: 'a',
        },
      });

      expect(store.attachments.value()?.map((item) => item.fileName)).toEqual(['nota.txt']);
    });

    it('stores activities and links and updates links for the shown issue', () => {
      store.fetchLinks('issue-1');
      httpMock.expectOne(`${BASE}/issue-1/links`).flush({ data: [] });

      store.createLink('issue-1', 'issue-9', 'relates').subscribe();
      httpMock.expectOne(`${BASE}/issue-1/links`).flush({
        data: {
          id: 'l-1',
          source_issue_id: 'issue-1',
          target_issue_id: 'issue-9',
          kind: 'relates',
          created_by: 'user-1',
          created_at: 'a',
        },
      });
      expect(store.links.value()?.map((link) => link.id)).toEqual(['l-1']);

      store.deleteLink('issue-1', 'l-1').subscribe();
      httpMock.expectOne(`${BASE}/issue-1/links/l-1`).flush(null);
      expect(store.links.value()).toEqual([]);

      store.fetchActivities('issue-1');
      httpMock.expectOne(`${BASE}/issue-1/activities`).flush({
        data: [
          {
            id: 'a-1',
            issue_id: 'issue-1',
            actor_id: 'user-1',
            action: 'assignee_changed',
            field_name: 'assignee_id',
            old_value: null,
            new_value: 'user-2',
            created_at: 'a',
          },
        ],
      });
      expect(store.activities.value()?.[0]?.action).toBe('assignee_changed');
      expect(store.activitiesIssueId()).toBe('issue-1');
    });
  });

  describe('list and details', () => {
    it('sends the filter and stores the page', () => {
      store.fetchTickets({ status: 'open', page: 1, pageSize: 20 });
      httpMock
        .expectOne((req) => req.url === BASE && req.params.get('status') === 'open')
        .flush({
          data: [issueDto()],
          meta: { page: 1, page_size: 20, total: 3 },
        });

      expect(store.tickets().map((item) => item.id)).toEqual(['issue-1']);
      expect(store.total()).toBe(3);
      expect(store.list.loading()).toBe(false);
    });

    it('keeps the list and exposes the error when the load fails', () => {
      store.fetchTickets();
      httpMock
        .expectOne(BASE)
        .flush(
          { error: { message: 'Sin acceso', code: 'issue.forbidden' } },
          { status: 403, statusText: 'Forbidden' },
        );

      expect(store.list.error()?.code).toBe('issue.forbidden');
      expect(store.list.error()?.message).toBe('Sin acceso');
      expect(store.tickets()).toEqual([]);
    });

    it('prepends a created ticket to a loaded list', () => {
      store.fetchTickets();
      httpMock
        .expectOne(BASE)
        .flush({ data: [issueDto()], meta: { page: 1, page_size: 20, total: 1 } });

      let created: Ticket | undefined;
      store
        .createTicket({ title: 'Nueva', description: '', priority: 'low' })
        .subscribe((item) => (created = item));
      httpMock.expectOne(BASE).flush({ data: issueDto({ id: 'issue-2', title: 'Nueva' }) });

      expect(created?.id).toBe('issue-2');
      expect(store.tickets().map((item) => item.id)).toEqual(['issue-2', 'issue-1']);
    });

    it('applies status, metadata and rank updates to the list and the selected issue', () => {
      store.fetchTickets();
      httpMock
        .expectOne(BASE)
        .flush({ data: [issueDto()], meta: { page: 1, page_size: 20, total: 1 } });
      store.fetchTicket('issue-1');
      httpMock.expectOne(`${BASE}/issue-1`).flush({ data: issueDto() });

      store.updateStatus('issue-1', 'resolved').subscribe();
      httpMock
        .expectOne(`${BASE}/issue-1/status`)
        .flush({ data: issueDto({ status: 'resolved' }) });
      expect(store.tickets()[0]?.status).toBe('resolved');
      expect(store.selected.value()?.status).toBe('resolved');

      store.updateRank('issue-1', 'z').subscribe();
      httpMock.expectOne(`${BASE}/issue-1/rank`).flush({ data: issueDto({ rank: 'z' }) });
      expect(store.selected.value()?.rank).toBe('z');

      store.updateMetadata('issue-1', { issueType: 'task' }).subscribe();
      httpMock
        .expectOne(`${BASE}/issue-1/metadata`)
        .flush({ data: issueDto({ issue_type: 'task' }) });
      expect(store.tickets()[0]?.issueType).toBe('task');
    });
  });

  describe('assign and unassign', () => {
    const issue = ticket();

    beforeEach(() => {
      store.list.set({ items: [issue], total: 1, page: 1, pageSize: 20 });
      store.selected.set(issue);
    });

    it('clears the assignee optimistically and stores the server answer', () => {
      store.unassign('issue-1').subscribe();

      expect(store.selected.value()).toMatchObject({ assigneeId: null, assigneeName: null });
      expect(store.tickets()[0]).toMatchObject({ assigneeId: null });

      const request = httpMock.expectOne(`${BASE}/issue-1/assign`);
      expect(request.request.body).toEqual({ assignee_id: null });
      request.flush({ data: issueDto({ assignee_id: null, assignee_name: null }) });

      expect(store.selected.value()?.assigneeId).toBeNull();
    });

    it('rolls back the previous assignee when clearing fails and reports the error', () => {
      let error: { message: string } | undefined;
      store
        .unassign('issue-1')
        .subscribe({ error: (value: { message: string }) => (error = value) });
      httpMock
        .expectOne(`${BASE}/issue-1/assign`)
        .flush({ error: { message: 'Sin conexión' } }, { status: 500, statusText: 'Server Error' });

      expect(error?.message).toBe('Sin conexión');
      expect(store.selected.value()).toMatchObject({ assigneeId: 'user-2', assigneeName: 'Luis' });
      expect(store.tickets()[0]).toMatchObject({ assigneeId: 'user-2' });
    });

    it('shows the new assignee optimistically and rolls back when assigning fails', () => {
      let failed = false;
      store.assign('issue-1', 'user-3').subscribe({ error: () => (failed = true) });

      expect(store.selected.value()).toMatchObject({ assigneeId: 'user-3', assigneeName: null });
      httpMock
        .expectOne(`${BASE}/issue-1/assign`)
        .flush({ error: { message: 'Prohibido' } }, { status: 403, statusText: 'Forbidden' });

      expect(failed).toBe(true);
      expect(store.selected.value()).toMatchObject({ assigneeId: 'user-2', assigneeName: 'Luis' });
    });

    it('stores the server answer when assigning succeeds', () => {
      store.assign('issue-1', 'user-3').subscribe();
      httpMock
        .expectOne(`${BASE}/issue-1/assign`)
        .flush({ data: issueDto({ assignee_id: 'user-3', assignee_name: 'Marta' }) });

      expect(store.selected.value()).toMatchObject({ assigneeId: 'user-3', assigneeName: 'Marta' });
    });

    it('does nothing until the assignment is subscribed', () => {
      store.assign('issue-1', 'user-3');

      expect(store.selected.value()).toMatchObject({ assigneeId: 'user-2' });
      httpMock.expectNone(`${BASE}/issue-1/assign`);
    });

    it('rolls back the list item when the issue was only in the list', () => {
      store.selected.set(undefined);
      let failed = false;
      store.unassign('issue-1').subscribe({ error: () => (failed = true) });
      httpMock
        .expectOne(`${BASE}/issue-1/assign`)
        .flush({ error: { message: 'x' } }, { status: 500, statusText: 'Server Error' });

      expect(failed).toBe(true);
      expect(store.tickets()[0]).toMatchObject({ assigneeId: 'user-2', assigneeName: 'Luis' });
    });
  });

  describe('member names', () => {
    function member(userId: string, name: string): MemberNameEntry {
      return { userId, name };
    }

    it('resolves ids to member names and falls back for ids that are not members', () => {
      listMembers = () => of([member('user-1', 'Ana García'), member('user-2', 'Luis Pérez')]);

      store.loadMemberNames('org-1');

      expect(store.memberNameOf('user-1')).toBe('Ana García');
      expect(store.memberNameOf('user-2')).toBe('Luis Pérez');
      expect(store.memberNameOf('removed-user')).toBe(UNKNOWN_MEMBER_NAME);
      expect(membersCalls).toEqual(['org-1']);
    });

    it('shows a loading placeholder until the member list arrives', () => {
      const response = new Subject<readonly MemberNameEntry[]>();
      listMembers = () => response;

      store.loadMemberNames('org-1');

      expect(store.memberNameOf('user-1')).toBe(LOADING_MEMBER_NAME);
      expect(store.membersLoading()).toBe(true);

      response.next([member('user-1', 'Ana García')]);
      response.complete();

      expect(store.memberNameOf('user-1')).toBe('Ana García');
      expect(store.membersLoading()).toBe(false);
    });

    it('requests the member list once per organization and shares it between callers', () => {
      listMembers = () => of([member('user-1', 'Ana García')]);

      store.loadMemberNames('org-1');
      store.loadMemberNames('org-1');

      expect(store.memberNameOf('user-1')).toBe('Ana García');
      expect(membersCalls).toHaveLength(1);
    });

    it('falls back to the unknown label when the member list cannot be loaded and retries on the next call', () => {
      listMembers = () => throwError(() => new Error('network failure'));
      store.loadMemberNames('org-1');

      expect(store.membersLoading()).toBe(false);
      expect(store.memberNameOf('user-1')).toBe(UNKNOWN_MEMBER_NAME);

      listMembers = () => of([member('user-1', 'Ana García')]);
      store.loadMemberNames('org-1');

      expect(store.memberNameOf('user-1')).toBe('Ana García');
      expect(membersCalls).toEqual(['org-1', 'org-1']);
    });

    it('does not request anything without an active organization', () => {
      store.loadMemberNames(null);

      expect(membersCalls).toEqual([]);
      expect(store.memberNameOf('user-1')).toBe(UNKNOWN_MEMBER_NAME);
      expect(store.membersLoading()).toBe(false);
    });

    it('loads the members of the new organization when the active one changes', () => {
      listMembers = (organizationId) =>
        of([member('user-1', organizationId === 'org-1' ? 'Ana García' : 'Ana G. (otra org)')]);

      store.loadMemberNames('org-1');
      expect(store.memberNameOf('user-1')).toBe('Ana García');

      store.loadMemberNames('org-2');

      expect(store.memberNameOf('user-1')).toBe('Ana G. (otra org)');
      expect(membersCalls).toEqual(['org-1', 'org-2']);
    });
  });

  it('clears the issue state on reset', () => {
    store.fetchTicket('issue-1');
    httpMock.expectOne(`${BASE}/issue-1`).flush({ data: issueDto() });

    store.reset();

    expect(store.selected.value()).toBeUndefined();
    expect(store.tickets()).toEqual([]);
    expect(store.total()).toBe(0);
  });
});
