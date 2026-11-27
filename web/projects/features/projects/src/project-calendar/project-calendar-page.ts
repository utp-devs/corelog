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
  CalendarEvent,
  CalendarEventKind,
  CalendarStore,
  isoDate,
  monthCells,
  projectAccessVariantOf,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge, LozengeTone } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { SelectButton } from 'primeng/selectbutton';

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

const KIND_TONES: Record<CalendarEventKind, LozengeTone> = {
  sprint_start: 'info',
  sprint_end: 'neutral',
  issue_due: 'warning',
};

const WEEKDAYS = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'];

const LOAD_FAILED_MESSAGE =
  'No se pudo cargar el calendario. Verifica que tengas acceso al proyecto e intenta de nuevo.';

/** Project milestones for one month: a month grid on wide screens and a date list on demand. */
@Component({
  selector: 'app-project-calendar-page',
  imports: [
    FormsModule,
    RouterLink,
    SelectButton,
    PageContainer,
    PageHeader,
    EmptyState,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
  ],
  templateUrl: './project-calendar-page.html',
  styleUrl: './project-calendar-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectCalendarPage implements OnInit {
  protected readonly calendar = inject(CalendarStore);
  protected readonly projectId = injectProjectId();

  protected readonly view = signal<CalendarView>('month');
  protected readonly viewOptions = VIEW_OPTIONS;
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

  protected kindTone(kind: CalendarEventKind): LozengeTone {
    return KIND_TONES[kind];
  }

  protected shift(months: number): void {
    this.calendar.shift(months);
  }

  protected retry(): void {
    this.calendar.retry();
  }
}
