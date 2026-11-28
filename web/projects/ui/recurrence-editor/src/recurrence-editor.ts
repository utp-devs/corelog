import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  MAX_RECURRENCE_COUNT,
  MAX_RECURRENCE_INTERVAL,
  NO_RECURRENCE,
  WEEKDAY_CODES,
  WEEKDAY_LABEL,
  WEEKDAY_SHORT,
  parseRecurrence,
  summarizeRecurrence,
  weekdayCodeOf,
  type RecurrenceFreq,
  type RecurrenceSpec,
  type WeekdayCode,
} from '@corelog/core';
import { DateField } from '@corelog/ui/date-field';
import { SelectField, type SelectFieldOption } from '@corelog/ui/select-field';

let nextRecurrenceEditorId = 0;

const FREQUENCIES: readonly SelectFieldOption[] = [
  { value: 'NONE', label: 'No se repite' },
  { value: 'DAILY', label: 'Cada día' },
  { value: 'WEEKLY', label: 'Cada semana' },
  { value: 'MONTHLY', label: 'Cada mes' },
  { value: 'YEARLY', label: 'Cada año' },
];

const UNIT_LABEL: Readonly<Record<RecurrenceFreq, string>> = {
  NONE: '',
  DAILY: 'días',
  WEEKLY: 'semanas',
  MONTHLY: 'meses',
  YEARLY: 'años',
};

/**
 * Builds a repetition rule: none, daily, weekly (weekday chips), monthly or yearly, every N units, ending
 * never, after N times or on a date. A sentence in Spanish says what the rule means.
 */
@Component({
  selector: 'app-recurrence-editor',
  imports: [DateField, FormsModule, SelectField],
  templateUrl: './recurrence-editor.html',
  styleUrl: './recurrence-editor.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RecurrenceEditor {
  readonly value = model<RecurrenceSpec>(NO_RECURRENCE);
  /** Day the event starts (AAAA-MM-DD): a new weekly rule starts on its weekday. */
  readonly startDate = input<string | null>(null);

  protected readonly uid = `recurrence-editor-${nextRecurrenceEditorId++}`;
  protected readonly frequencies = FREQUENCIES;
  protected readonly weekdays = WEEKDAY_CODES.map((code) => ({
    code,
    short: WEEKDAY_SHORT[code],
    label: WEEKDAY_LABEL[code],
  }));
  protected readonly maxInterval = MAX_RECURRENCE_INTERVAL;
  protected readonly maxCount = MAX_RECURRENCE_COUNT;
  protected readonly repeats = computed(() => this.value().freq !== 'NONE');
  protected readonly unit = computed(() => UNIT_LABEL[this.value().freq]);
  protected readonly summary = computed(() => summarizeRecurrence(this.value()));
  protected readonly endKind = computed(() => this.value().end.kind);
  protected readonly count = computed(() => {
    const end = this.value().end;
    return end.kind === 'count' ? end.count : 10;
  });
  protected readonly untilDate = computed(() => {
    const end = this.value().end;
    return end.kind === 'until' ? end.date : null;
  });

  /** Reads a rule from text, for hosts that keep the API string. */
  static fromRule(rule: string | null): RecurrenceSpec {
    return parseRecurrence(rule);
  }

  protected setFrequency(freq: string | null): void {
    const next = (freq ?? 'NONE') as RecurrenceFreq;
    let weekdays: readonly WeekdayCode[] = [];
    if (next === 'WEEKLY') {
      const kept = this.value().weekdays;
      const start = this.startDate();
      weekdays =
        kept.length > 0 ? kept : start ? [weekdayCodeOf(new Date(`${start}T00:00:00`))] : [];
    }
    this.value.set({ ...this.value(), freq: next, weekdays });
  }

  protected setInterval(raw: string): void {
    const parsed = Math.floor(Number(raw));
    const interval = Number.isFinite(parsed) ? Math.min(Math.max(parsed, 1), this.maxInterval) : 1;
    this.value.set({ ...this.value(), interval });
  }

  protected toggleWeekday(code: WeekdayCode): void {
    const current = this.value().weekdays;
    const weekdays = current.includes(code)
      ? current.filter((entry) => entry !== code)
      : [...current, code];
    // A weekly rule keeps at least one day.
    if (weekdays.length === 0) return;
    this.value.set({ ...this.value(), weekdays });
  }

  protected setEndKind(kind: 'never' | 'count' | 'until'): void {
    if (kind === 'never') this.value.set({ ...this.value(), end: { kind } });
    else if (kind === 'count')
      this.value.set({ ...this.value(), end: { kind, count: this.count() } });
    else {
      const start = this.startDate() ?? new Date().toISOString().slice(0, 10);
      this.value.set({ ...this.value(), end: { kind, date: this.untilDate() ?? start } });
    }
  }

  protected setCount(raw: string): void {
    const parsed = Math.floor(Number(raw));
    const count = Number.isFinite(parsed) ? Math.min(Math.max(parsed, 1), this.maxCount) : 1;
    this.value.set({ ...this.value(), end: { kind: 'count', count } });
  }

  protected setUntil(date: string | null): void {
    if (!date) return;
    this.value.set({ ...this.value(), end: { kind: 'until', date } });
  }
}
