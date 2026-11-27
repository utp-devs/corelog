import { Injectable, InjectionToken, computed, inject, signal } from '@angular/core';
import { Observable, catchError, defer, map, of, tap, throwError } from 'rxjs';

import { AttachmentApiService } from '../services/api/attachment-api.service';
import { IssueWatcherApiService } from '../services/api/issue-watcher-api.service';
import { TicketApiService } from '../services/api/ticket-api.service';
import {
  CreateTicketInput,
  IssueActivity,
  IssueAttachment,
  IssueComment,
  IssueLink,
  IssueLinkKind,
  IssueStatus,
  IssueWatcherState,
  ListTicketsFilter,
  Ticket,
  TicketPage,
  UpdateIssueDetailsInput,
  UpdateIssueMetadataInput,
} from '../models/ticket.model';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** One member of an organization, as needed to resolve ids to names. */
export interface MemberNameEntry {
  readonly userId: string;
  readonly name: string;
}

/**
 * Source of the member names of an organization. The organization feature provides the real one;
 * by default no member is known, so every id resolves to UNKNOWN_MEMBER_NAME.
 */
export interface MemberNamesSource {
  listMembers(organizationId: string): Observable<readonly MemberNameEntry[]>;
}

export const MEMBER_NAMES_SOURCE = new InjectionToken<MemberNamesSource>('MEMBER_NAMES_SOURCE', {
  providedIn: 'root',
  factory: () => ({ listMembers: () => of([]) }),
});

/** Name shown for an id that is not (or no longer) a member of the active organization. */
export const UNKNOWN_MEMBER_NAME = 'Usuario desconocido';
/** Placeholder shown while the member list of the organization is still loading. */
export const LOADING_MEMBER_NAME = 'Cargando…';

/**
 * Issue state: the paged list, the selected issue and the collaboration data of the issue
 * being viewed (comments, activities, links, watchers, attachments).
 * Each collection is tied to the issue it belongs to, so a late response for another issue is ignored.
 */
@Injectable({ providedIn: 'root' })
export class TicketStore {
  private readonly api = inject(TicketApiService);
  private readonly attachmentApi = inject(AttachmentApiService);
  private readonly watcherApi = inject(IssueWatcherApiService);
  private readonly memberSource = inject(MEMBER_NAMES_SOURCE);

  private readonly commentsIssue = signal<string | null>(null);
  private readonly activitiesIssue = signal<string | null>(null);
  private readonly linksIssue = signal<string | null>(null);
  private readonly watchersIssue = signal<string | null>(null);
  private readonly attachmentsIssue = signal<string | null>(null);
  private readonly memberOrganization = signal<string | null>(null);
  private readonly memberCache = new Map<string, ReadonlyMap<string, string>>();

  readonly list = new AsyncState<TicketPage>();
  readonly selected = new AsyncState<Ticket>();
  readonly comments = new AsyncState<IssueComment[]>();
  readonly activities = new AsyncState<IssueActivity[]>();
  readonly links = new AsyncState<IssueLink[]>();
  readonly watchers = new AsyncState<IssueWatcherState>();
  readonly attachments = new AsyncState<IssueAttachment[]>();
  readonly memberNames = new AsyncState<ReadonlyMap<string, string>>();

  readonly tickets = computed<readonly Ticket[]>(() => this.list.value()?.items ?? []);
  readonly total = computed(() => this.list.value()?.total ?? 0);
  readonly commentsIssueId = this.commentsIssue.asReadonly();
  readonly activitiesIssueId = this.activitiesIssue.asReadonly();
  readonly linksIssueId = this.linksIssue.asReadonly();
  readonly watchersIssueId = this.watchersIssue.asReadonly();
  readonly attachmentsIssueId = this.attachmentsIssue.asReadonly();
  readonly membersLoading = computed(
    () =>
      this.memberOrganization() !== null &&
      !this.memberNames.hasValue() &&
      this.memberNames.error() === null,
  );

  constructor() {
    resetOnLogout(() => this.reset());
  }

  fetchTickets(filter: ListTicketsFilter = {}): void {
    this.list.load(this.api.list(filter));
  }

  fetchTicket(id: string): void {
    this.selected.load(this.api.getById(id));
  }

  createTicket(input: CreateTicketInput): Observable<Ticket> {
    return this.api.create(input).pipe(
      tap((ticket) => {
        if (this.list.hasValue()) {
          this.list.update((page) => page && { ...page, items: [ticket, ...page.items] });
        }
      }),
    );
  }

  updateStatus(id: string, status: IssueStatus): Observable<Ticket> {
    return this.api.updateStatus(id, status).pipe(tap((updated) => this.applyUpdate(updated)));
  }

  /**
   * Assigns the issue (or clears the assignee with null). The new assignee shows at once; the previous
   * state comes back if the API rejects the change, and the error is re-thrown for the caller.
   */
  assign(id: string, assigneeId: string | null): Observable<Ticket> {
    return defer(() => {
      const previous = this.findTicket(id);
      if (previous) {
        // The name is unknown until the API answers, so it is cleared rather than shown stale.
        this.applyUpdate({ ...previous, assigneeId, assigneeName: null });
      }
      return this.api.assign(id, assigneeId).pipe(
        tap((updated) => this.applyUpdate(updated)),
        catchError((error: unknown) => {
          if (previous) {
            this.applyUpdate(previous);
          }
          return throwError(() => asApiError(error));
        }),
      );
    });
  }

  unassign(id: string): Observable<Ticket> {
    return this.assign(id, null);
  }

  updateMetadata(id: string, input: UpdateIssueMetadataInput): Observable<Ticket> {
    return this.api.updateMetadata(id, input).pipe(tap((updated) => this.applyUpdate(updated)));
  }

  updateDetails(id: string, input: UpdateIssueDetailsInput): Observable<Ticket> {
    return this.api.updateDetails(id, input).pipe(tap((updated) => this.applyUpdate(updated)));
  }

  updateRank(id: string, rank: string): Observable<Ticket> {
    return this.api.updateRank(id, rank).pipe(tap((updated) => this.applyUpdate(updated)));
  }

  fetchComments(id: string): void {
    this.comments.load(this.api.listComments(id).pipe(tap(() => this.commentsIssue.set(id))));
  }

  createComment(id: string, body: string): Observable<IssueComment> {
    return this.api.createComment(id, body).pipe(
      tap((comment) => {
        if (this.commentsIssue() === id) {
          this.comments.update((items) => [...(items ?? []), comment]);
        }
      }),
    );
  }

  deleteComment(issueId: string, commentId: string): Observable<void> {
    return this.api.deleteComment(issueId, commentId).pipe(
      tap(() => {
        if (this.commentsIssue() === issueId) {
          this.comments.update((items) => items?.filter((comment) => comment.id !== commentId));
        }
      }),
    );
  }

  fetchActivities(id: string): void {
    this.activities.load(this.api.listActivities(id).pipe(tap(() => this.activitiesIssue.set(id))));
  }

  fetchLinks(id: string): void {
    this.links.load(this.api.listLinks(id).pipe(tap(() => this.linksIssue.set(id))));
  }

  createLink(id: string, targetIssueId: string, kind: IssueLinkKind): Observable<IssueLink> {
    return this.api.createLink(id, targetIssueId, kind).pipe(
      tap((link) => {
        if (this.linksIssue() === id) {
          this.links.update((items) => [...(items ?? []), link]);
        }
      }),
    );
  }

  deleteLink(id: string, linkId: string): Observable<void> {
    return this.api.deleteLink(id, linkId).pipe(
      tap(() => {
        if (this.linksIssue() === id) {
          this.links.update((items) => items?.filter((link) => link.id !== linkId));
        }
      }),
    );
  }

  fetchWatchers(issueId: string): void {
    this.watchers.load(
      this.watcherApi.getState(issueId).pipe(tap(() => this.watchersIssue.set(issueId))),
    );
  }

  watch(issueId: string): Observable<IssueWatcherState> {
    return this.watcherApi.watch(issueId).pipe(tap((state) => this.setWatchers(issueId, state)));
  }

  unwatch(issueId: string): Observable<IssueWatcherState> {
    return this.watcherApi.unwatch(issueId).pipe(tap((state) => this.setWatchers(issueId, state)));
  }

  fetchAttachments(issueId: string): void {
    this.attachments.load(
      this.attachmentApi.list(issueId).pipe(tap(() => this.attachmentsIssue.set(issueId))),
    );
  }

  uploadAttachment(issueId: string, file: File): Observable<IssueAttachment> {
    return this.attachmentApi.upload(issueId, file).pipe(
      tap((attachment) => {
        if (this.attachmentsIssue() === issueId) {
          this.attachments.update((items) => [...(items ?? []), attachment]);
        }
      }),
    );
  }

  downloadAttachment(issueId: string, attachment: IssueAttachment): Observable<void> {
    return this.attachmentApi.download(issueId, attachment);
  }

  /**
   * Loads the member names of an organization once per organization. A failed load is not cached,
   * so the next call retries it. Passing null (no active organization) makes no request.
   */
  loadMemberNames(organizationId: string | null): void {
    if (organizationId === null) {
      this.memberOrganization.set(null);
      this.memberNames.reset();
      return;
    }
    if (
      this.memberOrganization() === organizationId &&
      (this.memberNames.hasValue() || this.memberNames.loading())
    ) {
      return;
    }
    this.memberOrganization.set(organizationId);
    this.memberNames.reset();
    const cached = this.memberCache.get(organizationId);
    if (cached) {
      this.memberNames.set(cached);
      return;
    }
    this.memberNames.load(
      this.memberSource.listMembers(organizationId).pipe(
        map(
          (entries) => new Map<string, string>(entries.map((entry) => [entry.userId, entry.name])),
        ),
        tap((names) => this.memberCache.set(organizationId, names)),
      ),
    );
  }

  /** Name of a user id in the active organization, or a placeholder while loading or when unknown. */
  memberNameOf(userId: string): string {
    const names = this.memberNames.value();
    if (names) {
      return names.get(userId) ?? UNKNOWN_MEMBER_NAME;
    }
    if (this.memberNames.error() !== null || this.memberOrganization() === null) {
      return UNKNOWN_MEMBER_NAME;
    }
    return LOADING_MEMBER_NAME;
  }

  reset(): void {
    this.list.reset();
    this.selected.reset();
    this.comments.reset();
    this.activities.reset();
    this.links.reset();
    this.watchers.reset();
    this.attachments.reset();
    this.memberNames.reset();
    this.commentsIssue.set(null);
    this.activitiesIssue.set(null);
    this.linksIssue.set(null);
    this.watchersIssue.set(null);
    this.attachmentsIssue.set(null);
    this.memberOrganization.set(null);
    this.memberCache.clear();
  }

  private setWatchers(issueId: string, state: IssueWatcherState): void {
    if (this.watchersIssue() === issueId) {
      this.watchers.set(state);
    }
  }

  private findTicket(id: string): Ticket | undefined {
    const selected = this.selected.value();
    if (selected?.id === id) {
      return selected;
    }
    return this.list.value()?.items.find((ticket) => ticket.id === id);
  }

  private applyUpdate(updated: Ticket): void {
    this.list.update(
      (page) =>
        page && {
          ...page,
          items: page.items.map((ticket) => (ticket.id === updated.id ? updated : ticket)),
        },
    );
    this.selected.update((current) => (current?.id === updated.id ? updated : current));
  }
}
