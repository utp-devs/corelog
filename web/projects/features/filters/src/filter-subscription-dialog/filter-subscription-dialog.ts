import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  AuthStore,
  FilterStore,
  NotificationService,
  asApiError,
  type FilterSubscription,
  type SavedQuery,
  type SubscriptionFrequency,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';
import { Select } from 'primeng/select';

const WEEKDAYS = [
  { value: 1, label: 'Lunes' },
  { value: 2, label: 'Martes' },
  { value: 3, label: 'Miércoles' },
  { value: 4, label: 'Jueves' },
  { value: 5, label: 'Viernes' },
  { value: 6, label: 'Sábado' },
  { value: 0, label: 'Domingo' },
];

const HOURS = Array.from({ length: 24 }, (_, hour) => ({
  value: hour,
  label: `${String(hour).padStart(2, '0')}:00`,
}));

const FREQUENCIES: Array<{ value: SubscriptionFrequency; label: string }> = [
  { value: 'daily', label: 'Cada día' },
  { value: 'weekly', label: 'Cada semana' },
];

/** Zones offered when the browser cannot list its own. */
function timezones(): string[] {
  const supported = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] })
    .supportedValuesOf;
  const own = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const list = supported ? supported('timeZone') : ['UTC', own];
  return list.includes(own) ? list : [own, ...list];
}

/** Schedule of the summary email of a filter: daily or weekly, hour and time zone. */
@Component({
  selector: 'app-filter-subscription-dialog',
  imports: [Button, Dialog, FormsModule, Select],
  templateUrl: './filter-subscription-dialog.html',
  styleUrl: './filter-subscription-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FilterSubscriptionDialog {
  private readonly store = inject(FilterStore);
  private readonly notify = inject(NotificationService);
  private readonly auth = inject(AuthStore);

  readonly open = input(false);
  readonly filter = input<SavedQuery | null>(null);
  readonly closeRequested = output<void>();
  readonly changed = output<void>();

  protected readonly frequency = signal<SubscriptionFrequency>('daily');
  protected readonly hour = signal(8);
  protected readonly weekday = signal(1);
  protected readonly timezone = signal('UTC');
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly subscribed = computed(() => this.filter()?.isSubscribed ?? false);
  protected readonly weekdays = WEEKDAYS;
  protected readonly hours = HOURS;
  protected readonly frequencies = FREQUENCIES;
  protected readonly zones = computed(() =>
    timezones().map((zone) => ({ value: zone, label: zone })),
  );

  constructor() {
    effect(() => {
      if (!this.open()) return;
      const current = untracked(() => this.filter()?.subscription ?? null);
      this.frequency.set(current?.frequency ?? 'daily');
      this.hour.set(current?.hour ?? 8);
      this.weekday.set(current?.weekday ?? 1);
      this.timezone.set(
        current?.timezone || untracked(() => this.auth.user()?.timezone) || timezones()[0],
      );
      this.error.set(null);
    });
  }

  protected async save(): Promise<void> {
    const filter = this.filter();
    if (!filter || this.busy()) return;
    const schedule: FilterSubscription = {
      frequency: this.frequency(),
      hour: this.hour(),
      weekday: this.frequency() === 'weekly' ? this.weekday() : 0,
      timezone: this.timezone(),
    };
    await this.run(() => this.store.subscribe(filter.id, schedule), 'Suscripción guardada');
  }

  protected async cancel(): Promise<void> {
    const filter = this.filter();
    if (!filter || this.busy()) return;
    await this.run(() => this.store.unsubscribe(filter.id), 'Suscripción cancelada');
  }

  private async run(action: () => Promise<void>, message: string): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      await action();
      this.notify.success(message);
      this.changed.emit();
      this.closeRequested.emit();
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.busy.set(false);
    }
  }
}
