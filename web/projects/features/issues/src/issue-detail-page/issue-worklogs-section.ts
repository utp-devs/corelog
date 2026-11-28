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
import { FormBuilder, ReactiveFormsModule } from '@angular/forms';
import {
  AuthStore,
  IssueViewStore,
  NotificationService,
  RemainingMode,
  Worklog,
  WorklogApiService,
  WorklogTotals,
  asApiError,
  parseDuration,
  MAX_DURATION_SECONDS,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { DateField } from '@corelog/ui/date-field';
import { Dialog } from '@corelog/ui/dialog';
import { DurationInput } from '@corelog/ui/duration-input';
import { FieldRow } from '@corelog/ui/field-row';
import { InlineEdit } from '@corelog/ui/inline-edit';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { Section, SectionActions } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

import { exactDate } from '../shared/time';

const MODE_OPTIONS: readonly { value: RemainingMode; label: string }[] = [
  { value: 'auto', label: 'Reducir automáticamente' },
  { value: 'set', label: 'Fijar un valor nuevo' },
  { value: 'reduce', label: 'Reducir en una cantidad' },
  { value: 'leave', label: 'No cambiar' },
];

function todayIso(): string {
  const now = new Date();
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

function nowTime(): string {
  const now = new Date();
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${pad(now.getHours())}:${pad(now.getMinutes())}`;
}

/**
 * Time tracking of one issue: estimates (editable inline), the time already logged with a progress
 * bar, the list of worklogs and the "Registrar tiempo" dialog (also used to edit a worklog).
 */
@Component({
  selector: 'app-issue-worklogs-section',
  imports: [
    AppTooltip,
    Button,
    ConfirmModal,
    DateField,
    Dialog,
    DurationInput,
    FieldRow,
    InlineEdit,
    LoadingSkeleton,
    Message,
    ProgressBar,
    ReactiveFormsModule,
    Section,
    SectionActions,
    SelectField,
  ],
  templateUrl: './issue-worklogs-section.html',
  styleUrl: './issue-worklogs-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueWorklogsSection {
  private readonly api = inject(WorklogApiService);
  private readonly viewStore = inject(IssueViewStore);
  private readonly auth = inject(AuthStore);
  private readonly notify = inject(NotificationService);
  private readonly fb = inject(FormBuilder);

  readonly issueId = input.required<string>();
  readonly archived = input(false);

  protected readonly worklogs = signal<readonly Worklog[]>([]);
  protected readonly totals = signal<WorklogTotals | null>(null);
  protected readonly loading = signal(true);
  protected readonly loadError = signal<string | null>(null);

  protected readonly dialogOpen = signal(false);
  protected readonly editing = signal<Worklog | null>(null);
  protected readonly saving = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly deleting = signal<Worklog | null>(null);

  protected readonly modeOptions = MODE_OPTIONS;
  protected readonly maxSeconds = MAX_DURATION_SECONDS;
  protected readonly exact = exactDate;

  protected readonly form = this.fb.nonNullable.group({
    timeSpent: '',
    date: todayIso(),
    time: nowTime(),
    description: '',
    mode: 'auto' as RemainingMode,
    value: '',
  });

  protected readonly canLog = computed(() => !this.archived() && this.viewStore.can('log_work'));
  protected readonly canEditEstimates = computed(
    () => !this.archived() && this.viewStore.can('edit_issue'),
  );
  protected readonly canManage = computed(() => this.viewStore.can('manage_worklogs'));
  protected readonly percent = computed(() => {
    const totals = this.totals();
    if (!totals) return 0;
    const base =
      (totals.originalEstimateSeconds ?? 0) ||
      totals.timeSpentSeconds + (totals.remainingEstimateSeconds ?? 0);
    return base > 0 ? (totals.timeSpentSeconds / base) * 100 : 0;
  });
  protected readonly showRemainingValue = computed(() => {
    const mode = this.form.controls.mode.value;
    return mode === 'set' || mode === 'reduce';
  });

  constructor() {
    effect(() => {
      const id = this.issueId();
      untracked(() => void this.load(id));
    });
  }

  protected canChange(worklog: Worklog): boolean {
    if (this.archived()) return false;
    return worklog.authorId === this.auth.user()?.id || this.canManage();
  }

  protected readonly saveOriginal = (next: string | null): Promise<void> =>
    this.saveEstimate('original', next);
  protected readonly saveRemaining = (next: string | null): Promise<void> =>
    this.saveEstimate('remaining', next);

  protected openLog(): void {
    this.editing.set(null);
    this.formError.set(null);
    this.form.reset({
      timeSpent: '',
      date: todayIso(),
      time: nowTime(),
      description: '',
      mode: 'auto',
      value: '',
    });
    this.dialogOpen.set(true);
  }

  protected openEdit(worklog: Worklog): void {
    const start = new Date(worklog.startedAt);
    const pad = (n: number): string => String(n).padStart(2, '0');
    this.editing.set(worklog);
    this.formError.set(null);
    this.form.reset({
      timeSpent: worklog.timeSpent,
      date: `${start.getFullYear()}-${pad(start.getMonth() + 1)}-${pad(start.getDate())}`,
      time: `${pad(start.getHours())}:${pad(start.getMinutes())}`,
      description: worklog.description,
      mode: 'auto',
      value: '',
    });
    this.dialogOpen.set(true);
  }

  protected closeDialog(): void {
    if (!this.saving()) this.dialogOpen.set(false);
  }

  protected async submit(): Promise<void> {
    const value = this.form.getRawValue();
    const seconds = parseDuration(value.timeSpent);
    if (seconds === null || seconds <= 0 || seconds > MAX_DURATION_SECONDS) {
      this.formError.set('Indica el tiempo dedicado, por ejemplo 1h 30m (máximo 3 días).');
      return;
    }
    if (this.showRemainingValue() && !this.editing() && parseDuration(value.value) === null) {
      this.formError.set('Indica la cantidad para la estimación restante.');
      return;
    }
    const startedAt = new Date(`${value.date}T${value.time || '00:00'}`);
    if (Number.isNaN(startedAt.getTime())) {
      this.formError.set('La fecha y la hora no son válidas.');
      return;
    }
    this.formError.set(null);
    this.saving.set(true);
    try {
      const editing = this.editing();
      if (editing) {
        await firstValueFrom(
          this.api.update(editing.id, {
            timeSpent: value.timeSpent.trim(),
            startedAt: startedAt.toISOString(),
            description: value.description.trim(),
          }),
        );
      } else {
        await firstValueFrom(
          this.api.log(this.issueId(), {
            timeSpent: value.timeSpent.trim(),
            startedAt: startedAt.toISOString(),
            description: value.description.trim() || undefined,
            remaining: {
              mode: value.mode,
              value: this.showRemainingValue() ? value.value.trim() : undefined,
            },
          }),
        );
      }
      this.dialogOpen.set(false);
      this.notify.success(editing ? 'Registro actualizado' : 'Tiempo registrado');
      await this.afterChange();
    } catch (error) {
      this.formError.set(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  protected async confirmDelete(): Promise<void> {
    const worklog = this.deleting();
    if (!worklog) return;
    this.saving.set(true);
    try {
      await firstValueFrom(this.api.remove(worklog.id));
      this.deleting.set(null);
      this.notify.success('Registro eliminado');
      await this.afterChange();
    } catch (error) {
      this.deleting.set(null);
      this.notify.error(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  private async saveEstimate(which: 'original' | 'remaining', next: string | null): Promise<void> {
    const text = next?.trim() ?? '';
    if (text !== '' && parseDuration(text) === null) {
      throw new Error('Usa semanas, días, horas y minutos, por ejemplo 1d 2h 30m.');
    }
    try {
      const totals = await firstValueFrom(
        this.api.setEstimates(this.issueId(), { [which]: text === '' ? null : text }),
      );
      this.totals.set(totals);
      void this.viewStore.refresh();
      this.notify.success('Estimación actualizada');
    } catch (error) {
      throw new Error(asApiError(error).message);
    }
  }

  private async afterChange(): Promise<void> {
    await this.load(this.issueId(), false);
    void this.viewStore.refresh();
  }

  private async load(issueId: string, showSkeleton = true): Promise<void> {
    if (showSkeleton) this.loading.set(true);
    this.loadError.set(null);
    try {
      const list = await firstValueFrom(this.api.list(issueId));
      if (issueId !== this.issueId()) return;
      this.worklogs.set(list.worklogs);
      this.totals.set(list.totals);
    } catch (error) {
      this.loadError.set(asApiError(error).message);
    } finally {
      this.loading.set(false);
    }
  }
}
