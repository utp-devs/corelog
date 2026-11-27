import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  Signal,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { AbstractControl, ReactiveFormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { InputNumber } from 'primeng/inputnumber';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';
import { Textarea } from 'primeng/textarea';
import { map } from 'rxjs';
import {
  NotificationService,
  OrganizationAdminStore,
  OrganizationStore,
  canEditOrgSettings,
  canUseOwnerDangerZone,
  timezoneOptions,
  warnBeforeUnload,
  type UnsavedChangesAware,
  type OrgWhoCan,
  type OrgDefaultVisibility,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { FieldRow, FieldControl } from '@corelog/ui/field-row';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';

import { adminErrorMessage } from '../shared/admin-error-message';
import { fieldMessage } from '../shared/field-message';
import { OrgDangerZone } from './org-danger-zone';
import { OrgLogoField } from './org-logo-field';
import {
  SettingsFormValues,
  applySettingsServerErrors,
  createSettingsForm,
  settingsFieldErrors,
  settingsPatchOf,
  settingsValuesOf,
} from './settings-form';

const WHO_CAN_OPTIONS: { label: string; value: OrgWhoCan }[] = [
  { value: 'members', label: 'Todos los miembros' },
  { value: 'admins', label: 'Solo administradores' },
];

const VISIBILITY_OPTIONS: { label: string; value: OrgDefaultVisibility }[] = [
  { value: 'open', label: 'Abierta: visible para todos los miembros' },
  { value: 'private', label: 'Privada: solo para quienes se añaden' },
];

const SECTION_LINKS = [
  { href: '#org-general', label: 'General' },
  { href: '#org-policies', label: 'Políticas' },
  { href: '#org-time', label: 'Tiempo' },
  { href: '#org-danger', label: 'Zona de peligro' },
] as const;

const BLANK_VALUES: SettingsFormValues = {
  name: '',
  description: '',
  whoCanCreateProjects: 'members',
  whoCanCreateTeams: 'members',
  defaultProjectVisibility: 'open',
  hoursPerDay: 8,
  daysPerWeek: 5,
  timezone: 'UTC',
};

/** Settings of the active organization. Owners and admins edit; other roles read. */
@Component({
  selector: 'app-org-settings-page',
  imports: [
    Button,
    ReactiveFormsModule,
    InputNumber,
    InputText,
    Message,
    Select,
    Textarea,
    EmptyState,
    FieldRow,
    FieldControl,
    Lozenge,
    PageContainer,
    PageHeader,
    Section,
    OrgDangerZone,
    OrgLogoField,
  ],
  templateUrl: './org-settings-page.html',
  styleUrl: './org-settings-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgSettingsPage implements UnsavedChangesAware {
  private readonly orgs = inject(OrganizationStore);
  private readonly admin = inject(OrganizationAdminStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly sectionLinks = SECTION_LINKS;
  protected readonly whoOptions = WHO_CAN_OPTIONS;
  protected readonly visibilityOptions = VISIBILITY_OPTIONS;

  protected readonly org = this.orgs.currentOrg;
  protected readonly role = this.orgs.currentRole;
  protected readonly canEdit = computed(() => canEditOrgSettings(this.role()));
  protected readonly ownerView = computed(() => canUseOwnerDangerZone(this.role()));
  protected readonly members = this.admin.members;
  protected readonly logoUrl = this.admin.logoUrl;
  protected readonly saving = signal(false);

  protected readonly form = createSettingsForm(BLANK_VALUES);
  protected readonly timezones = computed(() =>
    timezoneOptions(this.org()?.settings.timezone).map((zone) => ({ label: zone, value: zone })),
  );

  private readonly formDirty = toSignal(this.form.events.pipe(map(() => this.form.dirty)), {
    initialValue: false,
  });

  /** True while the form holds edits that have not been saved (read by the unsaved-changes guard). */
  readonly dirty: Signal<boolean> = this.formDirty;
  protected readonly dirtyAndEditable = computed(() => this.formDirty() && this.canEdit());

  private readonly versionKey = computed(() => {
    const current = this.org();
    return current ? `${current.id}|${current.updatedAt}` : null;
  });
  private readonly logoKey = computed(() => {
    const current = this.org();
    return current ? `${current.id}|${current.hasLogo}` : null;
  });

  constructor() {
    warnBeforeUnload(this.dirty);

    // A new organization or a refreshed `updatedAt` resets the form to the stored values.
    effect(() => {
      this.versionKey();
      untracked(() => {
        const current = this.org();
        if (current) this.form.reset(settingsValuesOf(current));
      });
    });

    effect(() => {
      const enabled = this.canEdit();
      untracked(() => {
        if (enabled) this.form.enable({ emitEvent: false });
        else this.form.disable({ emitEvent: false });
      });
    });

    effect(() => {
      this.logoKey();
      untracked(() => {
        const current = this.org();
        if (current) this.admin.loadLogo(current.id, current.hasLogo);
      });
    });

    effect(() => {
      this.logoKey();
      untracked(() => {
        const current = this.org();
        if (current && this.ownerView()) this.admin.fetchMembers(current.id);
      });
    });
  }

  /** Shows a rule message once the control is touched (blur or submit). */
  protected errorOf(control: AbstractControl): string | undefined {
    return control.touched ? (fieldMessage(control) ?? undefined) : undefined;
  }

  protected save(): void {
    const current = this.org();
    if (!current || !this.canEdit()) return;
    this.form.markAllAsTouched();
    if (this.form.invalid) return;

    this.saving.set(true);
    this.admin
      .updateOrganization(current.id, settingsPatchOf(this.form.getRawValue()))
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.saving.set(false);
          this.orgs.fetchOrgs();
          this.notify.success('Configuración guardada');
        },
        error: (error: unknown) => {
          this.saving.set(false);
          const fieldErrors = settingsFieldErrors(error);
          if (Object.keys(fieldErrors).length === 0) {
            this.notify.error(adminErrorMessage(error, 'No se pudo guardar la configuración.'));
          }
          applySettingsServerErrors(this.form, fieldErrors);
        },
      });
  }

  protected discard(): void {
    const current = this.org();
    if (current) this.form.reset(settingsValuesOf(current));
  }
}
