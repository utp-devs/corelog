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
import { FormsModule } from '@angular/forms';
import {
  NOTIFICATION_CHANNELS,
  NotificationSchemeStore,
  NotificationService,
  OrganizationApiService,
  ORGANIZATION_ID_PROVIDER,
  TeamStore,
  type NotificationChannel,
  type NotificationEvent,
  type NotificationRecipient,
  type NotificationSchemeScope,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { SelectField, type SelectFieldOption } from '@corelog/ui/select-field';
import { ToneTag } from '@corelog/ui/tone-tag';
import { UserPicker, type PickableUser } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { firstValueFrom } from 'rxjs';

import {
  CHANNEL_LABELS,
  EVENT_GROUPS,
  ROLE_LABELS,
  SIMPLE_RECIPIENTS,
  addRecipient,
  customize,
  isDirty,
  overrideCount,
  recipientKey,
  recipientLabel,
  removeRecipient,
  rowErrors,
  setChannel,
  toDraft,
  toInputs,
  type DraftRule,
} from './scheme-draft';

const ADD_PLACEHOLDER = 'Añadir destinatario';

const ADD_OPTIONS: readonly SelectFieldOption[] = [
  ...SIMPLE_RECIPIENTS.map((s) => ({ value: `simple:${s.type}`, label: s.label })),
  ...Object.entries(ROLE_LABELS).map(([role, label]) => ({ value: `role:${role}`, label })),
  { value: 'user', label: 'Una persona...' },
  { value: 'team', label: 'Un equipo...' },
];

type PickerMode = 'user' | 'team';

/**
 * Matrix of the notification scheme of a project or of the organization defaults: one row per event
 * with its channels and recipients. Rows that this scope does not own show the inherited values and a
 * "Heredado" badge; editing a row (or pressing "Personalizar") makes it the scope's own. Only owned rows
 * are saved, so the rest keep following the level above.
 */
@Component({
  selector: 'app-scheme-editor',
  imports: [
    Button,
    ConfirmModal,
    ErrorState,
    FormsModule,
    LoadingSkeleton,
    Message,
    SelectField,
    ToggleSwitch,
    ToneTag,
    UserPicker,
  ],
  providers: [NotificationSchemeStore],
  templateUrl: './scheme-editor.html',
  styleUrl: './scheme-editor.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SchemeEditor {
  protected readonly store = inject(NotificationSchemeStore);
  private readonly organizations = inject(OrganizationApiService);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly teams = inject(TeamStore);
  private readonly notices = inject(NotificationService);

  readonly scope = input.required<NotificationSchemeScope>();
  /** Whether the person may change the scheme; otherwise the matrix is read-only. */
  readonly canEdit = input(true);

  protected readonly groups = EVENT_GROUPS;
  protected readonly channels = NOTIFICATION_CHANNELS;
  protected readonly channelLabels = CHANNEL_LABELS;
  protected readonly addOptions = ADD_OPTIONS;
  protected readonly addPlaceholder = ADD_PLACEHOLDER;

  protected readonly draft = signal<readonly DraftRule[]>([]);
  protected readonly people = signal<readonly PickableUser[]>([]);
  protected readonly adding = signal<{ event: NotificationEvent; mode: PickerMode } | null>(null);
  protected readonly confirmReset = signal(false);
  protected readonly rowFailures = signal<ReadonlyMap<NotificationEvent, string>>(new Map());
  protected readonly generalFailure = signal<string | null>(null);

  protected readonly loading = computed(
    () => this.store.status() === 'idle' || this.store.status() === 'loading',
  );
  protected readonly dirty = computed(() => {
    const saved = this.store.scheme()?.rules;
    return saved ? isDirty(this.draft(), saved) : false;
  });
  protected readonly organizationScope = computed(() => this.scope().kind === 'organization');
  protected readonly ownedRows = computed(() => overrideCount(this.draft()));
  protected readonly canReset = computed(
    () => this.canEdit() && (this.store.scheme()?.rules.some((rule) => rule.overridden) ?? false),
  );
  protected readonly teamOptions = computed<readonly SelectFieldOption[]>(() =>
    this.teams.teams().map((team) => ({ value: team.id, label: team.name })),
  );
  protected readonly byEvent = computed(
    () => new Map(this.draft().map((rule) => [rule.event, rule] as const)),
  );

  constructor() {
    effect(() => {
      const scope = this.scope();
      untracked(() => {
        void this.reload(scope);
        void this.loadPeople();
      });
    });
    effect(() => {
      const scheme = this.store.scheme();
      untracked(() => {
        if (scheme) this.draft.set(toDraft(scheme.rules));
      });
    });
  }

  protected reload(scope: NotificationSchemeScope = this.scope()): Promise<void> {
    this.rowFailures.set(new Map());
    this.generalFailure.set(null);
    return this.store.load(scope);
  }

  private async loadPeople(): Promise<void> {
    const organization = this.organizationId();
    if (!organization) return;
    this.teams.fetchTeams();
    try {
      const members = await firstValueFrom(
        this.organizations.listMembers(organization, { silent: true }),
      );
      this.people.set(
        members.map((member) => ({
          id: member.userId,
          name: member.displayName || member.name,
          email: member.email,
        })),
      );
    } catch {
      // Without the directory a person chip falls back to a neutral label.
    }
  }

  protected rule(event: NotificationEvent): DraftRule | undefined {
    return this.byEvent().get(event);
  }

  protected hasChannel(event: NotificationEvent, channel: NotificationChannel): boolean {
    return this.rule(event)?.channels.includes(channel) ?? false;
  }

  protected toggleChannel(
    event: NotificationEvent,
    channel: NotificationChannel,
    on: boolean,
  ): void {
    this.draft.update((draft) => setChannel(draft, event, channel, on));
  }

  protected customizeRow(event: NotificationEvent): void {
    this.draft.update((draft) => customize(draft, event));
  }

  protected labelOf(recipient: NotificationRecipient): string {
    return recipientLabel(recipient, {
      person: (id) => this.people().find((p) => p.id === id)?.name ?? 'Persona de la organización',
      team: (id) => this.teams.teams().find((t) => t.id === id)?.name ?? 'eliminado',
    });
  }

  protected keyOf(recipient: NotificationRecipient): string {
    return recipientKey(recipient);
  }

  protected remove(event: NotificationEvent, recipient: NotificationRecipient): void {
    this.draft.update((draft) => removeRecipient(draft, event, recipient));
  }

  /** The add menu: simple and role recipients are added at once, people and teams open a picker. */
  protected choose(event: NotificationEvent, value: string | null): void {
    if (!value) return;
    if (value === 'user' || value === 'team') {
      this.adding.set({ event, mode: value });
      return;
    }
    const [kind, rest] = value.split(':');
    const recipient: NotificationRecipient =
      kind === 'role'
        ? { type: 'role', role: rest }
        : { type: rest as NotificationRecipient['type'] };
    this.draft.update((draft) => addRecipient(draft, event, recipient));
    this.adding.set(null);
  }

  protected addPerson(event: NotificationEvent, id: string | undefined): void {
    if (!id) return;
    this.draft.update((draft) => addRecipient(draft, event, { type: 'user', id }));
    this.adding.set(null);
  }

  protected addTeam(event: NotificationEvent, id: string | null): void {
    if (!id) return;
    this.draft.update((draft) => addRecipient(draft, event, { type: 'team', id }));
    this.adding.set(null);
  }

  protected cancelAdding(): void {
    this.adding.set(null);
  }

  protected discard(): void {
    const scheme = this.store.scheme();
    if (scheme) this.draft.set(toDraft(scheme.rules));
    this.rowFailures.set(new Map());
    this.generalFailure.set(null);
    this.adding.set(null);
  }

  protected async save(): Promise<void> {
    const inputs = toInputs(this.draft());
    this.rowFailures.set(new Map());
    this.generalFailure.set(null);
    if (await this.store.save(inputs)) {
      this.notices.success('Esquema de notificaciones guardado');
      return;
    }
    const failure = this.store.error();
    if (!failure) return;
    const mapped = rowErrors(failure, inputs);
    this.rowFailures.set(mapped.byEvent);
    this.generalFailure.set(mapped.general ?? 'Revisa las filas marcadas.');
  }

  protected async reset(): Promise<void> {
    this.confirmReset.set(false);
    this.rowFailures.set(new Map());
    this.generalFailure.set(null);
    if (await this.store.reset()) {
      this.notices.success(
        this.organizationScope()
          ? 'Se restableció el esquema base'
          : 'El proyecto vuelve a heredar el esquema de la organización',
      );
    } else {
      this.generalFailure.set(this.store.error()?.message ?? 'No se pudo restablecer el esquema.');
    }
  }
}
