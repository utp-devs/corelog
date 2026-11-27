import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import {
  FormControl,
  FormGroup,
  FormsModule,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import {
  AuthStore,
  NotificationService,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  OrganizationStore,
  Project,
  ProjectStore,
  ProjectTemplate,
  ProjectVisibility,
  SERVER_ERROR_KEY,
  TeamStore,
  applyServerErrors,
  asApiError,
  projectPath,
  warnBeforeUnload,
} from '@corelog/core';
import { ColorPicker } from '@corelog/ui/color-picker';
import { ErrorPage } from '@corelog/ui/error-page';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { PageContainer, PageSidebar } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { MultiSelect } from 'primeng/multiselect';
import { Textarea } from 'primeng/textarea';
import { Observable, catchError, of, switchMap } from 'rxjs';

import { createProjectRights } from '../projects-page/projects-page.utils';
import { PROJECT_TEMPLATES, ProjectTemplateInfo, templateInfo } from './project-templates';
import {
  KeyStatus,
  PROJECT_KEY_MAX_LENGTH,
  isValidKey,
  normalizeKey,
  projectColorCss,
  suggestKey,
} from './project-wizard.utils';

export interface WizardStep {
  readonly id: 'template' | 'details' | 'access' | 'review';
  readonly label: string;
}

export const WIZARD_STEPS: readonly WizardStep[] = [
  { id: 'template', label: 'Plantilla' },
  { id: 'details', label: 'Detalles' },
  { id: 'access', label: 'Acceso' },
  { id: 'review', label: 'Revisión' },
];

interface ChoiceOption<T extends string> {
  readonly value: T;
  readonly label: string;
  readonly description: string;
  readonly icon: string;
}

const VISIBILITY_CHOICES: readonly ChoiceOption<ProjectVisibility>[] = [
  {
    value: 'open',
    label: 'Abierto',
    description: 'Cualquier persona de la organización puede verlo.',
    icon: 'pi pi-globe',
  },
  {
    value: 'private',
    label: 'Privado',
    description: 'Solo lo ven las personas y equipos que agregues.',
    icon: 'pi pi-lock',
  },
];

const KEY_TAKEN_MESSAGE = 'Ya existe un proyecto con esta clave. Prueba con otra.';
const KEY_INVALID_MESSAGE =
  'Usa de 2 a 10 caracteres: una letra mayúscula y después letras o números.';
const KEY_CHECK_DELAY_MS = 350;
const NAME_MIN = 2;
const NAME_MAX = 120;
const DETAILS_STEP = 1;

let nextWizardId = 0;

/** Wizard that creates a project in four steps: template, details, access and review. */
@Component({
  selector: 'app-project-wizard',
  imports: [
    FormsModule,
    ReactiveFormsModule,
    RouterLink,
    Button,
    InputText,
    MultiSelect,
    Textarea,
    ColorPicker,
    ErrorPage,
    FieldControl,
    FieldRow,
    PageContainer,
    PageHeader,
    PageSidebar,
    UserPicker,
  ],
  templateUrl: './project-wizard.html',
  styleUrl: './project-wizard.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectWizard {
  private readonly store = inject(ProjectStore);
  private readonly teamStore = inject(TeamStore);
  private readonly auth = inject(AuthStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly orgApi = inject(OrganizationApiService);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly notify = inject(NotificationService);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly steps = WIZARD_STEPS;
  protected readonly templates = PROJECT_TEMPLATES;
  protected readonly visibilityChoices = VISIBILITY_CHOICES;
  protected readonly keyMaxLength = PROJECT_KEY_MAX_LENGTH;
  protected readonly idPrefix = `project-wizard-${nextWizardId++}`;

  protected readonly rights = computed(() =>
    createProjectRights(
      this.auth.user()?.role,
      this.orgs.currentRole(),
      this.orgs.currentOrg()?.settings.whoCanCreateProjects,
    ),
  );
  /** The organization role is read once the organization is known; until then nothing is decided. */
  protected readonly roleKnown = computed(
    () => this.orgs.currentRole() !== null || this.auth.user()?.role === 'admin',
  );

  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [
        Validators.required,
        Validators.minLength(NAME_MIN),
        Validators.maxLength(NAME_MAX),
      ],
    }),
    key: new FormControl('', { nonNullable: true }),
    description: new FormControl('', { nonNullable: true }),
    color: new FormControl<string | null>(null),
    visibility: new FormControl<ProjectVisibility>('open', { nonNullable: true }),
  });

  protected readonly step = signal(0);
  protected readonly furthest = signal(0);
  protected readonly template = signal<ProjectTemplate>('scrum');
  protected readonly templateChosen = signal(false);
  protected readonly leadId = signal<string | undefined>(undefined);
  protected readonly teamIds = signal<string[]>([]);
  protected readonly memberIds = signal<string[]>([]);
  protected readonly submitting = signal(false);
  protected readonly created = signal(false);
  protected readonly keyStatus = signal<KeyStatus>('idle');
  protected readonly detailsSubmitted = signal(false);
  protected readonly nameTouched = signal(false);
  protected readonly serverErrors = signal<Partial<Record<'name' | 'key', string>>>({});

  protected readonly people = signal<PickableUser[]>([]);
  protected readonly peopleLoading = signal(false);
  protected readonly teams = this.teamStore.teams;

  private readonly name = toSignal(this.form.controls.name.valueChanges, { initialValue: '' });
  private readonly key = toSignal(this.form.controls.key.valueChanges, { initialValue: '' });
  private readonly nameStatus = toSignal(this.form.controls.name.statusChanges, {
    initialValue: this.form.controls.name.status,
  });
  protected readonly color = toSignal(this.form.controls.color.valueChanges, {
    initialValue: null,
  });
  protected readonly visibility = toSignal(this.form.controls.visibility.valueChanges, {
    initialValue: 'open' as ProjectVisibility,
  });
  private readonly pristine = signal(true);

  protected readonly current = computed(() => this.steps[this.step()]);
  protected readonly templateData = computed<ProjectTemplateInfo>(() =>
    templateInfo(this.template()),
  );
  protected readonly displayName = computed(() => this.name().trim());
  protected readonly displayKey = computed(() => normalizeKey(this.key()));
  protected readonly leadName = computed(() => {
    const id = this.leadId();
    if (!id || id === this.auth.user()?.id) return this.selfName();
    return this.people().find((person) => person.id === id)?.name ?? 'Responsable elegido';
  });
  protected readonly selectedTeams = computed(() =>
    this.teams().filter((team) => this.teamIds().includes(team.id)),
  );
  protected readonly selectedPeople = computed(() =>
    this.people().filter((person) => this.memberIds().includes(person.id)),
  );
  protected readonly memberOptions = computed(() =>
    this.people().filter((person) => person.id !== (this.leadId() ?? this.auth.user()?.id)),
  );
  protected readonly teamNames = computed(() =>
    this.selectedTeams()
      .map((team) => team.name)
      .join(', '),
  );
  protected readonly peopleNames = computed(() =>
    this.selectedPeople()
      .map((person) => person.name)
      .join(', '),
  );
  protected readonly columnNames = computed(() =>
    this.templateData()
      .statuses.map((status) => status.name)
      .join(', '),
  );
  protected readonly swatchCss = computed(() => projectColorCss(this.color()));
  protected readonly visibilityLabel = computed(
    () => VISIBILITY_CHOICES.find((choice) => choice.value === this.visibility())?.label ?? '',
  );

  protected readonly nameError = computed<string | undefined>(() => {
    const server = this.serverErrors().name;
    if (server) return server;
    if (!this.nameTouched() && !this.detailsSubmitted()) return undefined;
    const control = this.form.controls.name;
    this.nameStatus();
    if (control.hasError('required')) return 'Escribe un nombre';
    if (control.hasError('minlength'))
      return `El nombre debe tener al menos ${NAME_MIN} caracteres`;
    if (control.hasError('maxlength')) {
      return `El nombre no puede tener más de ${NAME_MAX} caracteres`;
    }
    return undefined;
  });
  protected readonly keyError = computed<string | undefined>(() => {
    const server = this.serverErrors().key;
    if (server) return server;
    const status = this.keyStatus();
    if (status === 'taken') return KEY_TAKEN_MESSAGE;
    if (status === 'invalid') return KEY_INVALID_MESSAGE;
    if (this.detailsSubmitted() && this.displayKey() === '') return 'Escribe una clave';
    return undefined;
  });
  protected readonly keyHelper = computed(() => {
    switch (this.keyStatus()) {
      case 'checking':
        return 'Comprobando disponibilidad...';
      case 'available':
        return 'Clave disponible.';
      default:
        return 'Prefijo de las incidencias, por ejemplo CORE-12. Se sugiere a partir del nombre.';
    }
  });

  /** Edits that would be lost when leaving; false once the project exists. */
  readonly dirty = computed(
    () =>
      !this.created() &&
      (!this.pristine() ||
        this.step() > 0 ||
        this.templateChosen() ||
        this.teamIds().length > 0 ||
        this.memberIds().length > 0),
  );

  /** Why the current step cannot advance, or null when it can. */
  protected readonly blocker = computed<string | null>(() => {
    if (this.current().id !== 'details') return null;
    this.nameStatus();
    if (this.form.controls.name.invalid) return 'Escribe un nombre de al menos 2 caracteres.';
    const status = this.keyStatus();
    if (this.displayKey() === '' || status === 'invalid') return 'Escribe una clave válida.';
    if (status === 'taken') return 'La clave ya está en uso.';
    if (status === 'checking') return 'Espera a que termine la comprobación de la clave.';
    return null;
  });
  protected readonly canAdvance = computed(() => this.blocker() === null);

  /** True once the user typed a key: from then on the name no longer rewrites it. */
  private keyEdited = false;
  private keyTimer: number | undefined;
  private keyRequest = 0;

  constructor() {
    warnBeforeUnload(this.dirty);
    this.destroyRef.onDestroy(() => window.clearTimeout(this.keyTimer));

    this.form.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
      this.pristine.set(false);
    });
    this.form.controls.name.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((name) => {
        this.serverErrors.update((errors) => ({ ...errors, name: undefined }));
        if (!this.keyEdited) this.form.controls.key.setValue(suggestKey(name));
      });
    this.form.controls.key.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(() => this.serverErrors.update((errors) => ({ ...errors, key: undefined })));

    effect(() => {
      const key = this.displayKey();
      untracked(() => this.checkKey(key));
    });

    effect(() => {
      const id = this.auth.user()?.id;
      if (id && this.leadId() === undefined) untracked(() => this.leadId.set(id));
    });

    this.teamStore.fetchTeams();
    this.loadPeople();
  }

  protected stepId(index: number): string {
    return `${this.idPrefix}-step-${index}`;
  }

  protected stepState(index: number): 'done' | 'current' | 'todo' {
    if (index === this.step()) return 'current';
    return index < this.step() ? 'done' : 'todo';
  }

  protected canVisit(index: number): boolean {
    return index <= this.furthest() && index !== this.step() && !this.submitting();
  }

  protected goTo(index: number): void {
    if (!this.canVisit(index)) return;
    // Jumping forward is only allowed over steps that were already valid.
    if (index > this.step() && !this.canAdvance()) return;
    this.step.set(index);
  }

  protected next(): void {
    if (this.current().id === 'details') {
      this.detailsSubmitted.set(true);
      this.form.controls.name.markAsTouched();
    }
    if (!this.canAdvance()) return;
    const target = Math.min(this.step() + 1, this.steps.length - 1);
    this.step.set(target);
    this.furthest.update((value) => Math.max(value, target));
  }

  protected back(): void {
    this.step.update((value) => Math.max(0, value - 1));
  }

  protected chooseTemplate(value: ProjectTemplate): void {
    this.template.set(value);
    this.templateChosen.set(true);
  }

  protected isTemplate(value: ProjectTemplate): boolean {
    return this.template() === value;
  }

  protected onNameBlur(): void {
    this.nameTouched.set(true);
  }

  protected onKeyInput(value: string): void {
    const control = this.form.controls.key;
    this.keyEdited = value.trim() !== '';
    const upper = value.toUpperCase();
    if (upper !== value) control.setValue(upper);
    if (!this.keyEdited) control.setValue(suggestKey(this.form.controls.name.value));
  }

  protected setLead(id: string | undefined): void {
    const lead = id ?? this.auth.user()?.id;
    this.leadId.set(lead);
    this.memberIds.update((ids) => ids.filter((item) => item !== lead));
  }

  protected setTeamIds(ids: string[] | null): void {
    this.teamIds.set(ids ?? []);
  }

  protected setMemberIds(ids: string[] | null): void {
    this.memberIds.set(ids ?? []);
  }

  protected setVisibility(value: ProjectVisibility): void {
    this.form.controls.visibility.setValue(value);
  }

  protected submit(): void {
    if (this.submitting() || this.created() || !this.rights().allowed) return;
    this.submitting.set(true);
    const value = this.form.getRawValue();
    const info = this.templateData();
    const creatorId = this.auth.user()?.id;
    const leadId = this.leadId();
    const memberIds = this.memberIds().filter((id) => id !== (leadId ?? creatorId));

    this.store
      .createProject({
        key: normalizeKey(value.key),
        name: value.name.trim(),
        description: value.description.trim(),
        type: info.type,
        template: info.value,
        visibility: value.visibility,
        ...(value.color ? { color: value.color } : {}),
        ...(memberIds.length > 0 ? { memberIds } : {}),
        ...(this.teamIds().length > 0 ? { teamIds: this.teamIds() } : {}),
      })
      .pipe(
        switchMap((project) =>
          leadId && creatorId && leadId !== creatorId
            ? this.store
                .updateProject(project.id, {
                  name: project.name,
                  description: project.description,
                  type: project.type,
                  visibility: project.visibility,
                  leadId,
                })
                .pipe(catchError(() => this.leadNotAssigned(project)))
            : of(project),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (project) => {
          this.submitting.set(false);
          this.created.set(true);
          this.notify.success('Proyecto creado');
          void this.router.navigateByUrl(projectPath(project));
        },
        error: (error: unknown) => this.onCreateFailed(error),
      });
  }

  private onCreateFailed(error: unknown): void {
    this.submitting.set(false);
    const apiError = asApiError(error);
    const keyTaken = apiError.code === 'project.key_taken' || apiError.status === 409;
    const mapped = applyServerErrors(this.form, apiError);
    if (mapped) {
      const read = (field: 'name' | 'key'): string | undefined => {
        const control = this.form.controls[field];
        return control.hasError(SERVER_ERROR_KEY)
          ? String(control.getError(SERVER_ERROR_KEY))
          : undefined;
      };
      this.serverErrors.set({ name: read('name'), key: read('key') });
    }
    if (keyTaken && !this.serverErrors().key) {
      this.serverErrors.update((errors) => ({ ...errors, key: KEY_TAKEN_MESSAGE }));
      this.keyStatus.set('taken');
    }
    if (mapped || keyTaken) {
      this.step.set(DETAILS_STEP);
      return;
    }
    this.notify.error(apiError.message || 'No se pudo crear el proyecto');
  }

  /** The project exists but the chosen lead could not be set: say so and keep the creator as lead. */
  private leadNotAssigned(project: Project): Observable<Project> {
    this.notify.info('El proyecto se creó, pero no se pudo asignar a la persona responsable.');
    return of(project);
  }

  private selfName(): string {
    const user = this.auth.user();
    return user ? user.displayName || user.name : 'Tú';
  }

  private checkKey(key: string): void {
    window.clearTimeout(this.keyTimer);
    this.keyRequest += 1;
    if (key === '') {
      this.keyStatus.set('idle');
      return;
    }
    if (!isValidKey(key)) {
      this.keyStatus.set('invalid');
      return;
    }
    this.keyStatus.set('checking');
    const request = this.keyRequest;
    this.keyTimer = window.setTimeout(() => {
      this.store
        .keyAvailable(key)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe({
          next: (result) => {
            if (request !== this.keyRequest) return;
            this.keyStatus.set(result.available ? 'available' : 'taken');
          },
          // The check is a courtesy: when it fails the server decides at creation time.
          error: () => {
            if (request === this.keyRequest) this.keyStatus.set('unknown');
          },
        });
    }, KEY_CHECK_DELAY_MS);
  }

  private loadPeople(): void {
    const organizationId = this.organizationId();
    if (!organizationId) return;
    this.peopleLoading.set(true);
    this.orgApi
      .listMembers(organizationId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (members) => {
          this.people.set(
            members.map((member) => ({
              id: member.userId,
              name: member.displayName || member.name,
              email: member.email,
            })),
          );
          this.peopleLoading.set(false);
        },
        error: () => {
          this.people.set([]);
          this.peopleLoading.set(false);
        },
      });
  }
}
