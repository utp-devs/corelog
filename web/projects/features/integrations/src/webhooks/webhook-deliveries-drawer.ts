import { DatePipe } from '@angular/common';
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
import {
  NotificationService,
  WebhookApiService,
  asApiError,
  type Webhook,
  type WebhookDelivery,
  type WebhookDeliveryStatus,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { SidePanel } from '@corelog/ui/side-panel';
import { ToneTag } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';

import {
  DELIVERY_LABELS,
  DELIVERY_TONES,
  canRedeliver,
  eventLabel,
  prettyPayload,
} from './webhook-rules';

type Filter = 'all' | WebhookDeliveryStatus;
type LoadStatus = 'loading' | 'ready' | 'error';

/** Recent deliveries of a webhook: status filter, attempts, answer, readable payload and redelivery. */
@Component({
  selector: 'app-webhook-deliveries-drawer',
  imports: [Button, DatePipe, EmptyState, ErrorState, LoadingSkeleton, SidePanel, ToneTag],
  templateUrl: './webhook-deliveries-drawer.html',
  styleUrl: './webhook-deliveries-drawer.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WebhookDeliveriesDrawer {
  private readonly api = inject(WebhookApiService);
  private readonly notifications = inject(NotificationService);

  readonly webhook = input<Webhook | null>(null);
  readonly organizationId = input.required<string>();
  readonly closed = output<void>();

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly deliveries = signal<readonly WebhookDelivery[]>([]);
  protected readonly filter = signal<Filter>('all');
  protected readonly expanded = signal<ReadonlySet<string>>(new Set());
  protected readonly busy = signal<string | null>(null);

  protected readonly filters: readonly { value: Filter; label: string }[] = [
    { value: 'all', label: 'Todas' },
    { value: 'success', label: 'Correctas' },
    { value: 'failed', label: 'Fallidas' },
    { value: 'dead', label: 'Agotadas' },
    { value: 'pending', label: 'Pendientes' },
  ];
  protected readonly labels = DELIVERY_LABELS;
  protected readonly tones = DELIVERY_TONES;
  protected readonly canRedeliver = canRedeliver;
  protected readonly eventLabel = eventLabel;
  protected readonly prettyPayload = prettyPayload;

  protected readonly visible = computed(() => {
    const filter = this.filter();
    return filter === 'all'
      ? this.deliveries()
      : this.deliveries().filter((d) => d.status === filter);
  });

  constructor() {
    effect(() => {
      const hook = this.webhook();
      if (!hook) return;
      untracked(() => {
        this.filter.set('all');
        this.expanded.set(new Set());
        this.load();
      });
    });
  }

  protected load(): void {
    const hook = this.webhook();
    if (!hook) return;
    this.status.set('loading');
    this.api.deliveries(this.organizationId(), hook.id, 100).subscribe({
      next: (list) => {
        this.deliveries.set(list);
        this.status.set('ready');
      },
      error: () => this.status.set('error'),
    });
  }

  protected toggle(id: string): void {
    this.expanded.update((set) => {
      const next = new Set(set);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }

  protected async redeliver(delivery: WebhookDelivery): Promise<void> {
    const hook = this.webhook();
    if (!hook || this.busy()) return;
    this.busy.set(delivery.id);
    try {
      const queued = await firstValueFrom(
        this.api.redeliver(this.organizationId(), hook.id, delivery.id),
      );
      this.deliveries.update((list) => [queued, ...list]);
      this.notifications.success('El envío quedó en cola.');
    } catch (error) {
      this.notifications.error(asApiError(error));
    } finally {
      this.busy.set(null);
    }
  }
}
