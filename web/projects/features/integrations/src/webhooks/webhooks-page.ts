import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import {
  NotificationService,
  ORGANIZATION_ID_PROVIDER,
  ProjectStore,
  WebhookApiService,
  asApiError,
  type Webhook,
  type WebhookDelivery,
} from '@corelog/core';
import { ConfirmDangerModal } from '@corelog/ui/confirm-danger-modal';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Dialog } from '@corelog/ui/dialog';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { SecretReveal } from '@corelog/ui/secret-reveal';
import { ToneTag } from '@corelog/ui/tone-tag';
import { FormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { firstValueFrom } from 'rxjs';

import { WebhookDeliveriesDrawer } from './webhook-deliveries-drawer';
import { WebhookDialog, type ProjectOption } from './webhook-dialog';
import { DELIVERY_LABELS, DELIVERY_TONES, eventLabel, hostOf } from './webhook-rules';

type LoadStatus = 'loading' | 'ready' | 'error' | 'forbidden';

/**
 * Organization webhooks: the list with host, events, switch and last delivery; create, edit, test ping, secret
 * rotation, delivery history and deletion with a typed confirmation. Only owners and administrators reach it.
 */
@Component({
  selector: 'app-webhooks-page',
  imports: [
    Button,
    FormsModule,
    ConfirmDangerModal,
    ConfirmModal,
    DatePipe,
    Dialog,
    EmptyState,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    SecretReveal,
    ToggleSwitch,
    ToneTag,
    WebhookDeliveriesDrawer,
    WebhookDialog,
  ],
  templateUrl: './webhooks-page.html',
  styleUrl: './webhooks-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WebhooksPage {
  private readonly api = inject(WebhookApiService);
  private readonly notifications = inject(NotificationService);
  private readonly projectStore = inject(ProjectStore);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly webhooks = signal<readonly Webhook[]>([]);
  /** Last delivery known per webhook, from the history read after the list. */
  protected readonly lastDelivery = signal<Readonly<Record<string, WebhookDelivery | null>>>({});
  /** Result of the last test ping per webhook, shown inline. */
  protected readonly pings = signal<Readonly<Record<string, WebhookDelivery | 'error'>>>({});
  protected readonly pingError = signal<Readonly<Record<string, string>>>({});
  protected readonly testing = signal<string | null>(null);

  protected readonly formOpen = signal(false);
  protected readonly editing = signal<Webhook | null>(null);
  protected readonly history = signal<Webhook | null>(null);
  protected readonly deleting = signal<Webhook | null>(null);
  protected readonly deleteBusy = signal(false);
  protected readonly rotating = signal<Webhook | null>(null);
  protected readonly rotateBusy = signal(false);
  protected readonly rotatedSecret = signal<string | null>(null);

  protected readonly org = computed(() => this.organizationId());
  protected readonly projectOptions = computed<readonly ProjectOption[]>(() =>
    this.projectStore.projects().map((p) => ({ id: p.id, name: `${p.name} (${p.key})` })),
  );

  protected readonly labels = DELIVERY_LABELS;
  protected readonly tones = DELIVERY_TONES;
  protected readonly eventLabel = eventLabel;
  protected readonly hostOf = hostOf;

  constructor() {
    this.projectStore.fetchProjects();
    this.load();
  }

  protected eventsText(hook: Webhook): readonly string[] {
    return hook.events.length > 3
      ? [...hook.events.slice(0, 3).map(eventLabel), `+${hook.events.length - 3} más`]
      : hook.events.map(eventLabel);
  }

  protected load(): void {
    const org = this.org();
    if (!org) return;
    this.status.set('loading');
    this.api.list(org).subscribe({
      next: (list) => {
        this.webhooks.set(list);
        this.status.set('ready');
        this.loadLastDeliveries(org, list);
      },
      error: (error: unknown) =>
        this.status.set(asApiError(error).status === 403 ? 'forbidden' : 'error'),
    });
  }

  private loadLastDeliveries(org: string, list: readonly Webhook[]): void {
    for (const hook of list) {
      this.api.deliveries(org, hook.id, 1).subscribe({
        next: (items) => this.lastDelivery.update((m) => ({ ...m, [hook.id]: items[0] ?? null })),
        error: () => undefined,
      });
    }
  }

  protected openCreate(): void {
    this.editing.set(null);
    this.formOpen.set(true);
  }

  protected openEdit(hook: Webhook): void {
    this.editing.set(hook);
    this.formOpen.set(true);
  }

  protected onSaved(hook: Webhook): void {
    this.webhooks.update((list) =>
      list.some((h) => h.id === hook.id)
        ? list.map((h) => (h.id === hook.id ? hook : h))
        : [hook, ...list],
    );
  }

  protected async setEnabled(hook: Webhook, enabled: boolean): Promise<void> {
    const org = this.org();
    if (!org) return;
    try {
      const updated = await firstValueFrom(this.api.update(org, hook.id, { enabled }));
      this.onSaved(updated);
    } catch (error) {
      this.notifications.error(asApiError(error));
      this.webhooks.update((list) => list.map((h) => (h.id === hook.id ? { ...hook } : h)));
    }
  }

  protected async test(hook: Webhook): Promise<void> {
    const org = this.org();
    if (!org || this.testing()) return;
    this.testing.set(hook.id);
    this.pingError.update((m) => ({ ...m, [hook.id]: '' }));
    try {
      const delivery = await firstValueFrom(this.api.test(org, hook.id));
      this.pings.update((m) => ({ ...m, [hook.id]: delivery }));
      this.lastDelivery.update((m) => ({ ...m, [hook.id]: delivery }));
    } catch (error) {
      this.pings.update((m) => ({ ...m, [hook.id]: 'error' }));
      this.pingError.update((m) => ({ ...m, [hook.id]: asApiError(error).message }));
    } finally {
      this.testing.set(null);
    }
  }

  protected pingOf(hook: Webhook): WebhookDelivery | null {
    const ping = this.pings()[hook.id];
    return ping && ping !== 'error' ? ping : null;
  }

  protected async rotate(): Promise<void> {
    const org = this.org();
    const hook = this.rotating();
    if (!org || !hook || this.rotateBusy()) return;
    this.rotateBusy.set(true);
    try {
      this.rotatedSecret.set(await firstValueFrom(this.api.rotateSecret(org, hook.id)));
      this.rotating.set(null);
    } catch (error) {
      this.notifications.error(asApiError(error));
    } finally {
      this.rotateBusy.set(false);
    }
  }

  protected async remove(): Promise<void> {
    const org = this.org();
    const hook = this.deleting();
    if (!org || !hook || this.deleteBusy()) return;
    this.deleteBusy.set(true);
    try {
      await firstValueFrom(this.api.remove(org, hook.id));
      this.webhooks.update((list) => list.filter((h) => h.id !== hook.id));
      this.notifications.success(`Se eliminó el webhook «${hook.name}».`);
      this.deleting.set(null);
    } catch (error) {
      this.notifications.error(asApiError(error));
    } finally {
      this.deleteBusy.set(false);
    }
  }
}
