import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  EMAIL_OUTBOX_PAGE_SIZE,
  EmailOutboxStore,
  NotificationService,
  type EmailOutboxEntry,
  type EmailOutboxStatus,
} from '@corelog/core';
import {
  DataTable,
  DataTableCell,
  type DataTableColumn,
  type DataTablePageChange,
  type RowMenuItem,
} from '@corelog/ui/data-table';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { ToneTag } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { SelectButton } from 'primeng/selectbutton';
import { ToggleSwitch } from 'primeng/toggleswitch';

import {
  OUTBOX_STATUS_LABELS,
  OUTBOX_STATUS_TONES,
  exactTimeLabel,
  relativeTimeLabel,
} from './platform-format';

/** Seconds between automatic refreshes. */
export const OUTBOX_REFRESH_SECONDS = 10;

const COLUMNS: DataTableColumn[] = [
  { key: 'status', title: 'Estado', mobilePrimary: true },
  { key: 'subject', title: 'Asunto', field: 'subject', ellipsis: true, mobilePrimary: true },
  { key: 'to', title: 'Destinatario', field: 'to' },
  { key: 'kind', title: 'Tipo', field: 'kind' },
  { key: 'attempts', title: 'Intentos', field: 'attempts', align: 'right' },
  { key: 'created', title: 'Creado' },
  { key: 'next', title: 'Próximo intento' },
  { key: 'error', title: 'Último error', field: 'lastError', ellipsis: true },
];

const FILTERS: { label: string; value: EmailOutboxStatus | null }[] = [
  { label: 'Todos', value: null },
  { label: 'En cola', value: 'pending' },
  { label: 'Enviados', value: 'sent' },
  { label: 'Fallidos', value: 'dead' },
];

/** Outgoing email queue for platform administrators, with a status filter, retry and auto refresh. */
@Component({
  selector: 'app-email-outbox-page',
  imports: [
    Button,
    DataTable,
    DataTableCell,
    FormsModule,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    SelectButton,
    ToggleSwitch,
    ToneTag,
  ],
  templateUrl: './email-outbox-page.html',
  styleUrl: './email-outbox-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EmailOutboxPage {
  protected readonly store = inject(EmailOutboxStore);
  private readonly notices = inject(NotificationService);

  protected readonly columns = COLUMNS;
  protected readonly filters = FILTERS;
  protected readonly refreshSeconds = OUTBOX_REFRESH_SECONDS;
  protected readonly autoRefresh = signal(true);
  /** Clock behind the relative times; moves with every refresh. */
  protected readonly now = signal(new Date());

  protected readonly filter = computed(() => this.store.statusFilter());
  protected readonly pagination = computed(() => ({
    page: this.store.page(),
    pageSize: EMAIL_OUTBOX_PAGE_SIZE,
    total: this.store.total(),
  }));
  protected readonly rows = computed(() => this.store.items().slice());
  protected readonly loadError = computed(() => {
    const failure = this.store.error();
    return failure && !this.store.loaded() ? failure.message : null;
  });
  protected readonly refreshFailed = computed(
    () => this.store.error() !== null && this.store.loaded(),
  );

  constructor() {
    void this.store.load();
    effect((onCleanup) => {
      if (!this.autoRefresh()) return;
      const timer = setInterval(() => {
        this.now.set(new Date());
        void this.store.load(true);
      }, OUTBOX_REFRESH_SECONDS * 1000);
      onCleanup(() => clearInterval(timer));
    });
  }

  protected refresh(): void {
    this.now.set(new Date());
    void this.store.load();
  }

  protected setFilter(value: EmailOutboxStatus | null): void {
    void this.store.setFilter(value);
  }

  protected changePage(change: DataTablePageChange): void {
    void this.store.goTo(change.page);
  }

  protected statusLabel(row: EmailOutboxEntry): string {
    return OUTBOX_STATUS_LABELS[row.status];
  }

  protected statusTone(row: EmailOutboxEntry): 'neutral' | 'green' | 'red' {
    return OUTBOX_STATUS_TONES[row.status] as 'neutral' | 'green' | 'red';
  }

  protected ago(iso: string | null): string {
    return relativeTimeLabel(iso, this.now());
  }

  protected exact(iso: string | null): string {
    return exactTimeLabel(iso);
  }

  protected rowActions = (row: EmailOutboxEntry): RowMenuItem[] =>
    row.status === 'dead'
      ? [
          {
            key: 'retry',
            label: 'Reintentar',
            icon: 'pi pi-replay',
            primary: true,
            onClick: () => void this.retry(row),
          },
        ]
      : [];

  private async retry(row: EmailOutboxEntry): Promise<void> {
    const failure = await this.store.retry(row.id);
    if (failure?.status === 409) {
      this.notices.info('El correo ya no está en estado fallido.');
    } else if (failure) {
      this.notices.error(failure);
    } else {
      this.notices.success('El correo volvió a la cola');
    }
  }
}
