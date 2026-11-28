import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import {
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  Validators,
  type ValidatorFn,
} from '@angular/forms';
import {
  NO_RECURRENCE,
  addDays,
  allDayIso,
  browserTimeZone,
  buildRecurrence,
  isoDate,
  isoToZoned,
  parseIsoDate,
  parseRecurrence,
  timezoneOptions,
  zonedToIso,
  type CalendarEventDetail,
  type CalendarEventInput,
  type EventVisibility,
  type RecurrenceSpec,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { DateField } from '@corelog/ui/date-field';
import { FieldRow, FieldControl } from '@corelog/ui/field-row';
import { RecurrenceEditor } from '@corelog/ui/recurrence-editor';
import { SelectField, type SelectFieldOption } from '@corelog/ui/select-field';
import { UserPicker, type PickableUser } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Textarea } from 'primeng/textarea';

import { COLOR_OPTIONS, VISIBILITY_LABEL } from '../calendar-meta';

export type EventScope = 'series' | 'occurrence';

export interface EventSaveRequest {
  readonly input: CalendarEventInput;
  readonly scope: EventScope;
}

/** Where a new event starts, taken from the slot that was clicked. */
export interface EventStart {
  readonly date: string;
  readonly time: string;
  readonly allDay: boolean;
}

const TITLE_MAX = 200;
const LOCATION_MAX = 300;
const DESCRIPTION_MAX = 8000;
const MAX_ATTENDEES = 100;

const endAfterStart: ValidatorFn = (group) => {
  const value = group.value as {
    allDay: boolean;
    startDate: string | null;
    startTime: string;
    endDate: string | null;
    endTime: string;
  };
  if (!value.startDate || !value.endDate) return null;
  const start = `${value.startDate}T${value.allDay ? '00:00' : value.startTime}`;
  const end = `${value.endDate}T${value.allDay ? '00:00' : value.endTime}`;
  return end >= start && (value.allDay || end > start || value.endDate > value.startDate)
    ? null
    : { endBeforeStart: true };
};

/** Create or edit an event: title, place, dates and times, time zone, visibility, guests and repetition. */
@Component({
  selector: 'app-event-dialog',
  imports: [
    Button,
    DateField,
    Dialog,
    FieldControl,
    FieldRow,
    InputText,
    ReactiveFormsModule,
    RecurrenceEditor,
    SelectField,
    Textarea,
    UserPicker,
  ],
  templateUrl: './event-dialog.html',
  styleUrl: './event-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EventDialog {
  readonly open = input(false);
  /** The event being edited; null creates a new one. */
  readonly event = input<CalendarEventDetail | null>(null);
  /** Original instant of the occurrence the person opened, when the event repeats. */
  readonly occurrence = input<string | null>(null);
  /** Start and end of that occurrence, used when only the occurrence is edited. */
  readonly occurrenceTimes = input<{ readonly startsAt: string; readonly endsAt: string } | null>(
    null,
  );
  readonly start = input<EventStart | null>(null);
  /** Project a new event belongs to by default (calendar of one project). */
  readonly projectId = input<string | null>(null);
  readonly projects = input<readonly SelectFieldOption[]>([]);
  readonly people = input<readonly PickableUser[]>([]);
  readonly timezone = input<string>(browserTimeZone());
  readonly busy = input(false);
  readonly error = input<string | null>(null);

  readonly save = output<EventSaveRequest>();
  readonly closeRequested = output<void>();

  protected readonly titleMax = TITLE_MAX;
  protected readonly locationMax = LOCATION_MAX;
  protected readonly descriptionMax = DESCRIPTION_MAX;
  protected readonly colors = COLOR_OPTIONS;
  protected readonly visibilityOptions: readonly SelectFieldOption[] = (
    Object.keys(VISIBILITY_LABEL) as EventVisibility[]
  ).map((value) => ({ value, label: VISIBILITY_LABEL[value] }));

  protected readonly form = new FormGroup(
    {
      title: new FormControl('', {
        nonNullable: true,
        validators: [Validators.required, Validators.maxLength(TITLE_MAX)],
      }),
      description: new FormControl('', { nonNullable: true }),
      location: new FormControl('', { nonNullable: true }),
      allDay: new FormControl(false, { nonNullable: true }),
      startDate: new FormControl<string | null>(null, [Validators.required]),
      startTime: new FormControl('09:00', { nonNullable: true }),
      endDate: new FormControl<string | null>(null, [Validators.required]),
      endTime: new FormControl('10:00', { nonNullable: true }),
      timezone: new FormControl(browserTimeZone(), { nonNullable: true }),
      visibility: new FormControl<EventVisibility>('private', { nonNullable: true }),
      color: new FormControl('blue', { nonNullable: true }),
      projectId: new FormControl<string | null>(null),
    },
    { validators: [endAfterStart] },
  );

  protected readonly recurrence = signal<RecurrenceSpec>(NO_RECURRENCE);
  protected readonly attendeeIds = signal<readonly string[]>([]);
  protected readonly scope = signal<EventScope>('series');
  protected readonly submitted = signal(false);
  protected readonly pickerValue = signal<string | undefined>(undefined);

  private readonly allDayValue = toSignal(this.form.controls.allDay.valueChanges, {
    initialValue: false,
  });
  private readonly visibilityValue = toSignal(this.form.controls.visibility.valueChanges, {
    initialValue: 'private' as EventVisibility,
  });
  private readonly startDateValue = toSignal(this.form.controls.startDate.valueChanges, {
    initialValue: null as string | null,
  });
  private readonly statusValue = toSignal(this.form.statusChanges, { initialValue: 'INVALID' });

  protected readonly isEdit = computed(() => this.event() !== null);
  protected readonly isRecurring = computed(() => !!this.event()?.recurrenceRule);
  protected readonly occurrenceOnly = computed(
    () => this.isRecurring() && this.scope() === 'occurrence',
  );
  protected readonly heading = computed(() => (this.isEdit() ? 'Editar evento' : 'Nuevo evento'));
  protected readonly isAllDay = computed(() => this.allDayValue());
  protected readonly startDay = computed(() => this.startDateValue());
  protected readonly needsProject = computed(() => this.visibilityValue() === 'project');
  protected readonly zones = computed(() => timezoneOptions(this.form.controls.timezone.value));
  protected readonly zoneOptions = computed<SelectFieldOption[]>(() =>
    this.zones().map((zone) => ({ value: zone, label: zone })),
  );
  protected readonly attendees = computed(() => {
    const byId = new Map(this.people().map((person) => [person.id, person]));
    return this.attendeeIds().map((id) => ({
      id,
      name: byId.get(id)?.name ?? this.event()?.attendees.find((a) => a.userId === id)?.name ?? id,
    }));
  });
  protected readonly canAddAttendees = computed(() => this.attendeeIds().length < MAX_ATTENDEES);
  protected readonly invalid = computed(() => {
    this.statusValue();
    this.startDateValue();
    return this.form.invalid || (this.needsProject() && !this.form.controls.projectId.value);
  });
  protected readonly messages = {
    title: { required: 'Escribe un título.', maxlength: `Máximo ${TITLE_MAX} caracteres.` },
    date: { required: 'Elige una fecha.' },
  };

  constructor() {
    effect(() => {
      if (!this.open()) return;
      untracked(() => this.reset());
    });
    effect(() => {
      const picked = this.pickerValue();
      if (!picked) return;
      untracked(() => {
        if (!this.attendeeIds().includes(picked) && this.canAddAttendees()) {
          this.attendeeIds.update((ids) => [...ids, picked]);
        }
        this.pickerValue.set(undefined);
      });
    });
  }

  protected removeAttendee(id: string): void {
    this.attendeeIds.update((ids) => ids.filter((entry) => entry !== id));
  }

  protected setScope(scope: EventScope): void {
    this.scope.set(scope);
    this.fill();
  }

  protected submit(): void {
    this.submitted.set(true);
    if (this.busy()) return;
    this.form.markAllAsTouched();
    if (this.invalid()) return;
    const value = this.form.getRawValue();
    const zone = value.timezone || browserTimeZone();
    const startDate = value.startDate as string;
    const endDate = value.endDate as string;
    const times = value.allDay
      ? {
          startsAt: allDayIso(startDate),
          endsAt: allDayIso(isoDate(addDays(parseIsoDate(endDate), 1))),
        }
      : {
          startsAt: zonedToIso(startDate, value.startTime, zone),
          endsAt: zonedToIso(endDate, value.endTime, zone),
        };

    if (this.occurrenceOnly()) {
      this.save.emit({
        scope: 'occurrence',
        input: {
          title: value.title.trim(),
          description: value.description.trim(),
          location: value.location.trim(),
          color: value.color,
          ...times,
        },
      });
      return;
    }
    const input: CalendarEventInput = {
      title: value.title.trim(),
      description: value.description.trim(),
      location: value.location.trim(),
      allDay: value.allDay,
      timezone: zone,
      recurrenceRule: buildRecurrence(this.recurrence()),
      visibility: value.visibility,
      color: value.color,
      projectId: value.visibility === 'project' ? value.projectId : (value.projectId ?? null),
      attendees: this.attendeeIds(),
      ...times,
    };
    this.save.emit({ scope: 'series', input });
  }

  private reset(): void {
    this.submitted.set(false);
    this.scope.set(this.event() && this.occurrence() ? 'occurrence' : 'series');
    this.fill();
  }

  /** Fills the form from the event (series or occurrence) or from the clicked slot. */
  private fill(): void {
    const event = this.event();
    const zone = event?.timezone || this.timezone();
    const controls = this.form.controls;
    if (!event) {
      const start = this.start();
      const date = start?.date ?? isoDate(new Date());
      const time = start?.time ?? '09:00';
      const end = addHour(time);
      this.form.reset({
        title: '',
        description: '',
        location: '',
        allDay: start?.allDay ?? false,
        startDate: date,
        startTime: time,
        endDate: end.nextDay ? isoDate(addDays(parseIsoDate(date), 1)) : date,
        endTime: end.time,
        timezone: this.timezone(),
        visibility: this.projectId() ? 'project' : 'private',
        color: 'blue',
        projectId: this.projectId(),
      });
      this.recurrence.set(NO_RECURRENCE);
      this.attendeeIds.set([]);
      return;
    }
    const occurrenceTimes = this.occurrenceOnly() ? this.occurrenceTimes() : null;
    const startsAt = occurrenceTimes?.startsAt ?? event.startsAt;
    const endsAt = occurrenceTimes?.endsAt ?? event.endsAt;
    let startDate: string;
    let startTime: string;
    let endDate: string;
    let endTime: string;
    if (event.allDay) {
      startDate = startsAt.slice(0, 10);
      endDate = isoDate(addDays(parseIsoDate(endsAt.slice(0, 10)), -1));
      if (endDate < startDate) endDate = startDate;
      startTime = '09:00';
      endTime = '10:00';
    } else {
      const from = isoToZoned(startsAt, zone);
      const to = isoToZoned(endsAt, zone);
      startDate = from.date;
      startTime = from.time;
      endDate = to.date;
      endTime = to.time;
    }
    this.form.reset({
      title: event.title,
      description: event.description,
      location: event.location,
      allDay: event.allDay,
      startDate,
      startTime,
      endDate,
      endTime,
      timezone: zone,
      visibility: event.visibility,
      color: event.color || 'blue',
      projectId: event.projectId,
    });
    this.recurrence.set(parseRecurrence(event.recurrenceRule));
    this.attendeeIds.set(event.attendees.map((attendee) => attendee.userId));
    controls.title.markAsPristine();
  }
}

/** One hour after a HH:mm time; `nextDay` is true when it wraps past midnight. */
function addHour(time: string): { time: string; nextDay: boolean } {
  const [hour, minute] = time.split(':').map(Number);
  const next = hour + 1;
  return {
    time: `${String(next % 24).padStart(2, '0')}:${String(minute).padStart(2, '0')}`,
    nextDay: next >= 24,
  };
}
