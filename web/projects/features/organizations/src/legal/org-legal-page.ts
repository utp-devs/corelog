import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import {
  COMMUNICATIONS_SLUG,
  LegalApiService,
  LegalStore,
  NotificationService,
  asApiError,
  type ConsentRecord,
  type LegalDocument,
  type PublishLegalPayload,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { DataTable, DataTableColumn, RowMenuItem } from '@corelog/ui/data-table';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Section } from '@corelog/ui/section';
import { SelectField, type SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { SelectButton } from 'primeng/selectbutton';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { firstValueFrom } from 'rxjs';

import { formatOrgDate } from '../shared/org-format';

const SLUGS: readonly SelectFieldOption[] = [
  { value: 'privacy_policy', label: 'Política de privacidad' },
  { value: 'data_processing', label: 'Tratamiento de datos personales' },
  { value: 'terms_of_use', label: 'Términos de uso' },
  { value: 'cookies', label: 'Política de cookies' },
  { value: 'communications', label: 'Comunicaciones del producto' },
];
const SLUG_LABELS = new Map(SLUGS.map((o) => [o.value as string, o.label]));
const REQUIRED_BY_DEFAULT = new Set(['privacy_policy', 'data_processing', 'terms_of_use']);

const SOURCE_LABELS: Readonly<Record<string, string>> = {
  register: 'Registro',
  invite: 'Invitación',
  login_reprompt: 'Nueva versión',
  profile: 'Perfil',
};

const DOCUMENT_COLUMNS: DataTableColumn[] = [
  { key: 'title', title: 'Documento', field: 'title', mobilePrimary: true },
  { key: 'version', title: 'Versión vigente', field: 'version', mobilePrimary: true },
  { key: 'effective', title: 'Vigente desde', field: 'effective' },
  { key: 'required', title: 'Obligatorio', field: 'required' },
  { key: 'versions', title: 'Versiones publicadas', field: 'versions' },
];

const CONSENT_COLUMNS: DataTableColumn[] = [
  { key: 'user', title: 'Usuario', field: 'user', ellipsis: true, mobilePrimary: true },
  { key: 'document', title: 'Documento', field: 'document', mobilePrimary: true },
  { key: 'version', title: 'Versión', field: 'version' },
  { key: 'date', title: 'Fecha', field: 'date' },
  { key: 'source', title: 'Origen', field: 'source' },
  { key: 'state', title: 'Estado', field: 'state' },
];

interface DocumentRow {
  readonly slug: string;
  readonly title: string;
  readonly version: number;
  readonly effective: string;
  readonly required: string;
  readonly versions: number;
}

interface ConsentRow {
  readonly id: string;
  readonly user: string;
  readonly document: string;
  readonly version: number;
  readonly date: string;
  readonly source: string;
  readonly state: string;
}

const VIEW_OPTIONS = [
  { label: 'Editar', value: 'edit' },
  { label: 'Vista previa', value: 'preview' },
] as const;

/**
 * Platform administration of the legal documents: the current version of each one, a publisher for a
 * new version (Markdown editor with preview, confirmation that users will be asked to accept again) and
 * the audit list of the consents users gave.
 */
@Component({
  selector: 'app-org-legal-page',
  imports: [
    Button,
    ConfirmModal,
    DataTable,
    Dialog,
    FieldControl,
    FieldRow,
    FormsModule,
    InputText,
    PageContainer,
    PageHeader,
    ReactiveFormsModule,
    RichTextView,
    Section,
    SelectButton,
    SelectField,
    ToggleSwitch,
  ],
  templateUrl: './org-legal-page.html',
  styleUrl: './org-legal-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgLegalPage {
  private readonly api = inject(LegalApiService);
  private readonly legal = inject(LegalStore);
  private readonly notifications = inject(NotificationService);

  protected readonly documentColumns = DOCUMENT_COLUMNS;
  protected readonly consentColumns = CONSENT_COLUMNS;
  protected readonly slugOptions = SLUGS;
  protected readonly viewOptions = VIEW_OPTIONS;

  protected readonly loading = signal(true);
  protected readonly loadError = signal<string | null>(null);
  protected readonly versions = signal<readonly LegalDocument[]>([]);

  protected readonly consentsLoading = signal(false);
  protected readonly consentsError = signal<string | null>(null);
  protected readonly records = signal<readonly ConsentRecord[]>([]);
  protected readonly userFilter = new FormControl('', { nonNullable: true });

  // Publisher state.
  protected readonly publishing = signal(false);
  protected readonly slug = signal<string>('privacy_policy');
  protected readonly slugLocked = signal(false);
  protected readonly title = signal('');
  protected readonly summary = signal('');
  protected readonly body = signal('');
  protected readonly required = signal(true);
  protected readonly view = signal<'edit' | 'preview'>('edit');
  protected readonly confirming = signal(false);
  protected readonly busy = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly preparing = signal(false);

  protected readonly rows = computed<readonly DocumentRow[]>(() => {
    const bySlug = new Map<string, LegalDocument[]>();
    for (const doc of this.versions()) bySlug.set(doc.slug, [...(bySlug.get(doc.slug) ?? []), doc]);
    return [...bySlug.entries()].map(([slug, docs]) => {
      const current = docs.reduce((a, b) => (b.version > a.version ? b : a));
      return {
        slug,
        title: current.title,
        version: current.version,
        effective: formatOrgDate(current.effectiveFrom),
        required: current.required ? 'Sí' : 'No',
        versions: docs.length,
      };
    });
  });

  protected readonly consentRows = computed<readonly ConsentRow[]>(() =>
    this.records().map((r) => ({
      id: r.id,
      user: r.userId,
      document: SLUG_LABELS.get(r.slug) ?? r.slug,
      version: r.version,
      date: formatOrgDate(r.acceptedAt),
      source: SOURCE_LABELS[r.source] ?? r.source,
      state: r.revokedAt ? 'Revocado' : 'Vigente',
    })),
  );

  protected readonly nextVersion = computed(() => {
    const current = this.rows().find((r) => r.slug === this.slug());
    return (current?.version ?? 0) + 1;
  });
  protected readonly requiredLocked = computed(() => this.slug() === COMMUNICATIONS_SLUG);
  protected readonly canPublish = computed(
    () => this.title().trim().length > 0 && this.body().trim().length > 0,
  );
  protected readonly confirmText = computed(() => {
    const label = SLUG_LABELS.get(this.slug()) ?? this.slug();
    const effect = this.required()
      ? 'Todas las personas tendrán que aceptarla de nuevo la próxima vez que entren.'
      : 'Es un documento opcional: nadie tendrá que aceptarlo de nuevo.';
    return `Vas a publicar la versión ${this.nextVersion()} de «${label}». Entra en vigor de inmediato. ${effect}`;
  });

  protected readonly rowActionsFor = (row: DocumentRow): RowMenuItem[] => [
    {
      key: 'publish',
      label: 'Publicar nueva versión',
      icon: 'pi pi-upload',
      primary: true,
      onClick: () => void this.openPublisher(row.slug),
    },
  ];

  constructor() {
    void this.load();
    void this.loadConsents();
  }

  protected async load(): Promise<void> {
    this.loading.set(true);
    this.loadError.set(null);
    try {
      this.versions.set(await firstValueFrom(this.api.adminListDocuments()));
    } catch (error) {
      this.loadError.set(asApiError(error).message);
    } finally {
      this.loading.set(false);
    }
  }

  protected async loadConsents(): Promise<void> {
    this.consentsLoading.set(true);
    this.consentsError.set(null);
    try {
      const user = this.userFilter.value.trim();
      this.records.set(await firstValueFrom(this.api.adminConsents(user || undefined)));
    } catch (error) {
      this.consentsError.set(asApiError(error).message);
      this.records.set([]);
    } finally {
      this.consentsLoading.set(false);
    }
  }

  protected clearFilter(): void {
    this.userFilter.setValue('');
    void this.loadConsents();
  }

  /** Opens the publisher; an existing document brings its current text so the edit starts from it. */
  protected async openPublisher(slug?: string): Promise<void> {
    this.formError.set(null);
    this.view.set('edit');
    this.slugLocked.set(slug !== undefined);
    this.slug.set(slug ?? 'privacy_policy');
    this.title.set('');
    this.summary.set('');
    this.body.set('');
    this.required.set(REQUIRED_BY_DEFAULT.has(this.slug()));
    this.publishing.set(true);
    if (!slug) return;
    this.preparing.set(true);
    try {
      const current = await this.legal.read(slug);
      this.title.set(current.title);
      this.summary.set(current.summary);
      this.body.set(current.bodyMarkdown);
      this.required.set(current.required);
    } catch {
      this.formError.set(
        'No se pudo cargar el texto vigente. Puedes escribir el nuevo desde cero.',
      );
    } finally {
      this.preparing.set(false);
    }
  }

  protected onSlug(value: string): void {
    this.slug.set(value);
    this.required.set(value !== COMMUNICATIONS_SLUG && REQUIRED_BY_DEFAULT.has(value));
  }

  protected closePublisher(): void {
    this.publishing.set(false);
    this.confirming.set(false);
  }

  protected async publish(): Promise<void> {
    const payload: PublishLegalPayload = {
      slug: this.slug(),
      title: this.title().trim(),
      summary: this.summary().trim(),
      bodyMarkdown: this.body(),
      required: this.requiredLocked() ? false : this.required(),
    };
    this.busy.set(true);
    this.formError.set(null);
    try {
      const published = await firstValueFrom(this.api.adminPublish(payload));
      this.confirming.set(false);
      this.publishing.set(false);
      this.notifications.success(`Versión ${published.version} publicada`);
      await this.legal.load(true);
      await this.load();
    } catch (error) {
      this.confirming.set(false);
      this.formError.set(asApiError(error).message || 'No se pudo publicar el documento.');
    } finally {
      this.busy.set(false);
    }
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement | HTMLTextAreaElement).value;
  }
}
