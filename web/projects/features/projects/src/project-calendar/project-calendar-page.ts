import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import {
  BreakpointService,
  CalendarEvent,
  CalendarEventKind,
  CalendarStore,
  isoDate,
  monthCells,
  PROJECT_MOBILE_QUERY,
  projectAccessVariantOf,
} from '@corelog/core';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { AppTooltip } from '@corelog/ui/tooltip';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { SelectButton } from 'primeng/selectbutton';

import { shortKeyOf } from '../shared/issue-key';
import { injectProjectId } from '../shared/project-route';

type CalendarView = 'month' | 'list';

const VIEW_OPTIONS: { label: string; value: CalendarView }[] = [
  { label: 'Mes', value: 'month' },
  { label: 'Lista', value: 'list' },
];

const KIND_LABELS: Record<CalendarEventKind, string> = {
  sprint_start: 'Inicio de sprint',
  sprint_end: 'Fin de sprint',
  issue_due: 'Fecha límite',
};

const KIND_ICONS: Record<CalendarEventKind, string> = {
  sprint_start: 'pi pi-play',
  sprint_end: 'pi pi-flag',
  issue_due: 'pi pi-clock',
};

const WEEKDAYS = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'];

const LOAD_FAILED_MESSAGE =
  'No se pudo cargar el calendario. Verifica que tengas acceso al proyecto e intenta de nuevo.';

/** Project milestones for one month: a month grid on wide screens and a date list on demand. */
@Component({
  selector: 'app-project-calendar-page',
  imports: [
    FormsModule,
    NgTemplateOutlet,
    RouterLink,
    SelectButton,
    AppTooltip,
    PageContainer,
    PageHeader,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
  ],
  templateUrl: './project-calendar-page.html',
  styleUrl: './project-calendar-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectCalendarPage implements OnInit {
  protected readonly calendar = inject(CalendarStore);
  protected readonly projectId = injectProjectId();
  /** Phones always get the agenda: a month grid does not fit there. */
  protected readonly isMobile = inject(BreakpointService).matches(PROJECT_MOBILE_QUERY);

  protected readonly view = signal<CalendarView>('month');
  protected readonly viewOptions = VIEW_OPTIONS;
  protected readonly activeView = computed<CalendarView>(() =>
    this.isMobile() ? 'list' : this.view(),
  );
  protected readonly today = isoDate(new Date());
  protected readonly weekdays = WEEKDAYS;
  protected readonly loadFailedMessage = LOAD_FAILED_MESSAGE;

  protected readonly monthLabel = computed(() =>
    this.calendar.month().toLocaleDateString('es-ES', { month: 'long', year: 'numeric' }),
  );
  protected readonly cells = computed(() => monthCells(this.calendar.month()));

  protected readonly failed = computed(() => this.calendar.events.error() !== null);
  protected readonly accessVariant = computed(() => {
    const error = this.calendar.events.error();
    return error ? projectAccessVariantOf(error) : null;
  });
  // Shown while a month is requested, so a previous month never stays on the grid.
  protected readonly loading = computed(
    () => !this.failed() && (this.calendar.events.loading() || !this.calendar.events.hasValue()),
  );

  private loadedId: string | null = null;

  constructor() {
    // Follows route changes that reuse this component (another project). The first load runs in ngOnInit.
    effect(() => {
      const id = this.projectId();
      untracked(() => this.load(id));
    });
  }

  ngOnInit(): void {
    this.load(this.projectId());
  }

  private load(id: string): void {
    if (id === this.loadedId) return;
    this.loadedId = id;
    this.calendar.reset();
    this.calendar.load(id);
  }

  protected eventsOn(cell: Date): readonly CalendarEvent[] {
    return this.calendar.eventsByDate()[isoDate(cell)] ?? [];
  }

  protected kindLabel(kind: CalendarEventKind): string {
    return KIND_LABELS[kind];
  }

  protected kindIcon(kind: CalendarEventKind): string {
    return KIND_ICONS[kind];
  }

  protected shortKey(issueId: string): string {
    return shortKeyOf(issueId);
  }

  protected isToday(cell: Date): boolean {
    return isoDate(cell) === this.today;
  }

  /** Day of the agenda as a Spanish date, for example "jueves 5 de noviembre". */
  protected formatListDate(iso: string): string {
    const [year, month, day] = iso.split('-').map(Number);
    return new Date(year, month - 1, day).toLocaleDateString('es-ES', {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    });
  }

  protected shift(months: number): void {
    this.calendar.shift(months);
  }

  protected retry(): void {
    this.calendar.retry();
  }
}
