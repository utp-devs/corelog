import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import {
  ALL_KIND_GROUPS,
  BreakpointService,
  CalendarV2Store,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  PROJECT_MOBILE_QUERY,
  ProjectStore,
  asApiError,
  browserTimeZone,
  isoDate,
  localDateOf,
  localTimeOf,
  viewTitle,
  type CalendarEventDetail,
  type CalendarFeedSecret,
  type CalendarItem,
  type CalendarKindGroup,
  type CalendarViewMode,
  type RsvpResponse,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Dialog } from '@corelog/ui/dialog';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import type { SelectFieldOption } from '@corelog/ui/select-field';
import { AppTooltip } from '@corelog/ui/tooltip';
import type { PickableUser } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { MultiSelect } from 'primeng/multiselect';
import { SelectButton } from 'primeng/selectbutton';
import { firstValueFrom } from 'rxjs';

import { CalendarAgenda } from '../calendar-agenda/calendar-agenda';
import { GROUP_META } from '../calendar-meta';
import { CalendarMonth } from '../calendar-month/calendar-month';
import { CalendarTimeGrid } from '../calendar-time-grid/calendar-time-grid';
import { EventDialog, type EventSaveRequest, type EventStart } from '../event-dialog/event-dialog';
import { EventPanel } from '../event-panel/event-panel';
import { FeedDialog } from '../feed-dialog/feed-dialog';

const DESKTOP_VIEWS: { label: string; value: CalendarViewMode }[] = [
  { label: 'Mes', value: 'month' },
  { label: 'Semana', value: 'week' },
  { label: 'Día', value: 'day' },
  { label: 'Agenda', value: 'agenda' },
];
const MOBILE_VIEWS = DESKTOP_VIEWS.filter(
  (view) => view.value === 'agenda' || view.value === 'day',
);

const LOAD_FAILED = 'No se pudo cargar el calendario. Inténtalo de nuevo.';

/**
 * The calendar: toolbar (navigation, view, filters, new event, ICS subscription), the month, week, day or agenda
 * view, the details panel of an event and the dialogs to create, edit, delete and subscribe. Pass a project to
 * limit the derived dates (issues, sprints, versions) to it and to default new events to it.
 */
@Component({
  selector: 'app-calendar-view',
  imports: [
    AppTooltip,
    Button,
    CalendarAgenda,
    CalendarMonth,
    CalendarTimeGrid,
    ConfirmModal,
    Dialog,
    ErrorState,
    EventDialog,
    EventPanel,
    FeedDialog,
    FormsModule,
    LoadingSkeleton,
    MultiSelect,
    SelectButton,
  ],
  templateUrl: './calendar-view.html',
  styleUrl: './calendar-view.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarView {
  protected readonly store = inject(CalendarV2Store);
  private readonly router = inject(Router);
  private readonly projectStore = inject(ProjectStore);
  private readonly organizations = inject(OrganizationApiService);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly isMobile = inject(BreakpointService).matches(PROJECT_MOBILE_QUERY);

  /** Limits the calendar to one project (the project calendar). */
  readonly projectId = input<string | null>(null);

  protected readonly viewOptions = computed(() => (this.isMobile() ? MOBILE_VIEWS : DESKTOP_VIEWS));
  protected readonly groups = ALL_KIND_GROUPS;
  protected readonly groupMeta = GROUP_META;
  protected readonly title = computed(() => viewTitle(this.store.view(), this.store.anchor()));
  protected readonly loadFailed = LOAD_FAILED;
  protected readonly failed = computed(() => this.store.window.error() !== null);
  protected readonly firstLoad = computed(() => !this.failed() && !this.store.window.hasValue());
  protected readonly projectOptions = computed<SelectFieldOption[]>(() =>
    this.projectStore
      .projects()
      .map((project) => ({ value: project.id, label: `${project.key} · ${project.name}` })),
  );
  protected readonly selectedProjects = signal<string[]>([]);

  // Details panel.
  protected readonly panelItem = signal<CalendarItem | null>(null);
  protected readonly panelDetail = signal<CalendarEventDetail | null>(null);
  protected readonly panelLoading = signal(false);
  protected readonly panelError = signal<string | null>(null);
  protected readonly panelBusy = signal(false);

  // Create and edit.
  protected readonly dialogOpen = signal(false);
  protected readonly dialogStart = signal<EventStart | null>(null);
  protected readonly dialogEvent = signal<CalendarEventDetail | null>(null);
  protected readonly dialogBusy = signal(false);
  protected readonly dialogError = signal<string | null>(null);
  protected readonly people = signal<readonly PickableUser[]>([]);
  protected readonly timezone = browserTimeZone();

  // Delete.
  protected readonly deleteOpen = signal(false);
  protected readonly deleteBusy = signal(false);
  protected readonly deleteError = signal<string | null>(null);

  // ICS feed.
  protected readonly feedOpen = signal(false);
  protected readonly feedSecret = signal<CalendarFeedSecret | null>(null);
  protected readonly feedBusy = signal(false);
  protected readonly feedError = signal<string | null>(null);

  protected readonly occurrence = computed(() => this.panelItem()?.occurrenceStart ?? null);
  protected readonly occurrenceTimes = computed(() => {
    const item = this.panelItem();
    return item?.recurring ? { startsAt: item.startsAt, endsAt: item.endsAt } : null;
  });
  protected readonly isRecurring = computed(() => !!this.panelDetail()?.recurrenceRule);

  private membersLoaded = false;

  constructor() {
    effect(() => {
      const project = this.projectId();
      untracked(() => {
        this.selectedProjects.set(project ? [project] : []);
        this.applyInitialView();
        this.store.setProjects(project ? [project] : []);
      });
    });
    effect(() => {
      // The window follows the screen: phones list the agenda, wide screens never keep it by accident.
      const mobile = this.isMobile();
      untracked(() => {
        const view = this.store.view();
        if (mobile && (view === 'month' || view === 'week')) this.store.setView('agenda');
      });
    });
    this.projectStore.fetchProjects();
  }

  private applyInitialView(): void {
    if (this.isMobile() && this.store.view() !== 'day') this.store.setView('agenda');
    else if (!this.isMobile() && this.store.view() === 'agenda') this.store.setView('month');
  }

  protected setView(view: CalendarViewMode): void {
    this.store.setView(view);
  }

  protected setProjects(ids: string[]): void {
    this.selectedProjects.set(ids);
    this.store.setProjects(ids);
  }

  protected isGroupOn(group: CalendarKindGroup): boolean {
    return this.store.groups().includes(group);
  }

  protected openDay(date: Date): void {
    this.store.setView('day');
    this.store.setAnchor(date);
  }

  // Selecting.

  protected select(item: CalendarItem): void {
    if (item.kind !== 'event') {
      if (item.link) void this.router.navigateByUrl(item.link);
      return;
    }
    this.panelItem.set(item);
    this.panelDetail.set(null);
    this.panelError.set(null);
    this.panelLoading.set(true);
    void this.store
      .fetchEvent(item.sourceId)
      .then((detail) => {
        if (this.panelItem() === item) this.panelDetail.set(detail);
      })
      .catch((error: unknown) => this.panelError.set(asApiError(error).message))
      .finally(() => this.panelLoading.set(false));
  }

  protected closePanel(): void {
    this.panelItem.set(null);
    this.panelDetail.set(null);
  }

  protected async respond(response: RsvpResponse): Promise<void> {
    const item = this.panelItem();
    if (!item) return;
    this.panelBusy.set(true);
    try {
      await this.store.respond(item.sourceId, response);
      const detail = await this.store.fetchEvent(item.sourceId);
      this.panelDetail.set(detail);
      this.panelError.set(null);
    } catch (error) {
      this.panelError.set(asApiError(error).message);
    } finally {
      this.panelBusy.set(false);
    }
  }

  // Create and edit.

  protected newEvent(date?: Date, allDay = false): void {
    const day = date ?? this.store.anchor();
    const hasTime = date !== undefined && !allDay && date.getHours() > 0;
    this.openDialog(null, {
      date: isoDate(day),
      time: hasTime ? localTimeOf(day.toISOString()) : '09:00',
      allDay,
    });
  }

  protected editEvent(): void {
    const detail = this.panelDetail();
    if (!detail) return;
    this.openDialog(detail, null);
  }

  private openDialog(event: CalendarEventDetail | null, start: EventStart | null): void {
    this.dialogEvent.set(event);
    this.dialogStart.set(start);
    this.dialogError.set(null);
    this.dialogOpen.set(true);
    void this.loadMembers();
  }

  protected closeDialog(): void {
    this.dialogOpen.set(false);
  }

  private async loadMembers(): Promise<void> {
    const organization = this.organizationId();
    if (this.membersLoaded || !organization) return;
    try {
      const members = await firstValueFrom(
        this.organizations.listMembers(organization, { silent: true }),
      );
      this.people.set(
        members.map((member) => ({ id: member.userId, name: member.name, email: member.email })),
      );
      this.membersLoaded = true;
    } catch {
      // Guests are optional: the dialog still works without the directory.
    }
  }

  protected async saveEvent(request: EventSaveRequest): Promise<void> {
    this.dialogBusy.set(true);
    this.dialogError.set(null);
    const event = this.dialogEvent();
    const item = this.panelItem();
    try {
      if (!event) {
        await this.store.create({
          ...request.input,
          organizationId: this.organizationId(),
        });
      } else if (request.scope === 'occurrence' && item?.occurrenceStart) {
        await this.store.update(event.id, request.input, item.occurrenceStart);
      } else {
        await this.store.update(event.id, request.input);
      }
      this.dialogOpen.set(false);
      this.closePanel();
    } catch (error) {
      this.dialogError.set(asApiError(error).message);
    } finally {
      this.dialogBusy.set(false);
    }
  }

  // Delete.

  protected askDelete(): void {
    this.deleteError.set(null);
    this.deleteOpen.set(true);
  }

  protected cancelDelete(): void {
    this.deleteOpen.set(false);
  }

  protected async confirmDelete(scope: 'series' | 'occurrence'): Promise<void> {
    const item = this.panelItem();
    if (!item) return;
    this.deleteBusy.set(true);
    try {
      await this.store.remove(
        item.sourceId,
        scope === 'occurrence' ? (item.occurrenceStart ?? undefined) : undefined,
      );
      this.deleteOpen.set(false);
      this.closePanel();
    } catch (error) {
      this.deleteError.set(asApiError(error).message);
    } finally {
      this.deleteBusy.set(false);
    }
  }

  // ICS feed.

  protected openFeed(): void {
    this.feedSecret.set(null);
    this.feedError.set(null);
    this.feedOpen.set(true);
    this.store.loadFeed();
  }

  protected async generateFeed(): Promise<void> {
    this.feedBusy.set(true);
    this.feedError.set(null);
    try {
      const secret = await this.store.createFeed();
      // The API answers with a path; the address pasted into another app must be complete.
      const url = secret.url.startsWith('/')
        ? new URL(secret.url, window.location.origin).toString()
        : secret.url;
      this.feedSecret.set({ ...secret, url });
    } catch (error) {
      this.feedError.set(asApiError(error).message);
    } finally {
      this.feedBusy.set(false);
    }
  }

  protected async revokeFeed(): Promise<void> {
    this.feedBusy.set(true);
    this.feedError.set(null);
    try {
      await this.store.revokeFeed();
      this.feedSecret.set(null);
    } catch (error) {
      this.feedError.set(asApiError(error).message);
    } finally {
      this.feedBusy.set(false);
    }
  }

  protected dayOfItem(item: CalendarItem): string {
    return localDateOf(item.startsAt);
  }
}
