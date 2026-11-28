import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import {
  AutomationApiService,
  CurrentProjectStore,
  NotificationService,
  OrganizationApiService,
  StatusCatalogStore,
  TeamStore,
  asApiError,
  projectPath,
  type AutomationRule,
  type AutomationRuleDraft,
  type AutomationValidation,
} from '@corelog/core';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import {
  RuleBuilder,
  emptyRule,
  validateRule,
  type RuleProblem,
  type RuleTarget,
} from '@corelog/ui/rule-builder';
import { Section } from '@corelog/ui/section';
import { FormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { firstValueFrom, map } from 'rxjs';

type LoadStatus = 'loading' | 'ready' | 'error' | 'forbidden';

function draftOf(rule: AutomationRule): AutomationRuleDraft {
  return {
    name: rule.name,
    enabled: rule.enabled,
    trigger: rule.trigger,
    conditions: rule.conditions,
    actions: rule.actions,
  };
}

/**
 * Builder page for one rule (new or existing): name, the three-part definition, a live summary, a dry validation
 * that lists the planned actions, and saving with a prompt to reload when someone else changed the rule first.
 */
@Component({
  selector: 'app-rule-editor-page',
  imports: [
    Button,
    FormsModule,
    ErrorState,
    InputText,
    LoadingSkeleton,
    RouterLink,
    RuleBuilder,
    Section,
    ToggleSwitch,
  ],
  templateUrl: './rule-editor-page.html',
  styleUrl: './rule-editor-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RuleEditorPage {
  private readonly api = inject(AutomationApiService);
  private readonly current = inject(CurrentProjectStore);
  private readonly notifications = inject(NotificationService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly orgApi = inject(OrganizationApiService);
  private readonly statusCatalog = inject(StatusCatalogStore);
  private readonly teamStore = inject(TeamStore);

  private readonly ruleId = toSignal(this.route.paramMap.pipe(map((p) => p.get('ruleId'))), {
    initialValue: this.route.snapshot.paramMap.get('ruleId'),
  });

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly rule = signal<AutomationRuleDraft>(emptyRule());
  protected readonly loaded = signal<AutomationRule | null>(null);
  protected readonly people = signal<readonly RuleTarget[]>([]);
  protected readonly showProblems = signal(false);
  protected readonly serverCheck = signal<AutomationValidation | null>(null);
  protected readonly serverError = signal<string | null>(null);
  protected readonly validating = signal(false);
  protected readonly saving = signal(false);
  protected readonly conflict = signal(false);

  protected readonly isNew = computed(() => this.ruleId() === null);
  protected readonly archived = this.current.isArchived;
  protected readonly canEdit = computed(
    () => this.current.can('manage_automation') && !this.archived(),
  );
  protected readonly problems = computed<readonly RuleProblem[]>(() => validateRule(this.rule()));
  protected readonly shownProblems = computed(() => (this.showProblems() ? this.problems() : []));
  protected readonly nameProblem = computed(
    () => this.shownProblems().find((p) => p.path === 'name')?.message ?? null,
  );
  protected readonly statuses = computed(() => this.statusCatalog.statuses().map((s) => s.name));
  protected readonly teams = computed<readonly RuleTarget[]>(() =>
    this.teamStore.teams().map((t) => ({ id: t.id, name: t.name })),
  );
  protected readonly backLink = computed(() => {
    const project = this.current.project();
    return project ? projectPath(project, 'settings/automation') : '.';
  });

  constructor() {
    this.statusCatalog.ensureLoaded();
    this.teamStore.fetchTeams();

    effect(() => {
      const projectId = this.current.projectId();
      const org = this.current.project()?.organizationId;
      if (!projectId || !org || !this.current.permissionsLoaded()) return;
      const id = this.ruleId();
      untracked(() => {
        this.loadPeople(org);
        if (!this.current.can('manage_automation')) {
          this.status.set('forbidden');
        } else if (id) {
          this.loadRule(id);
        } else {
          this.rule.set(emptyRule());
          this.loaded.set(null);
          this.status.set('ready');
        }
      });
    });
  }

  private loadPeople(org: string): void {
    this.orgApi.listMembers(org).subscribe({
      next: (members) =>
        this.people.set(
          members.map((m) => ({ id: m.userId, name: m.displayName || m.name || m.email })),
        ),
      error: () => undefined,
    });
  }

  protected loadRule(id: string): void {
    this.status.set('loading');
    this.api.get(id).subscribe({
      next: (rule) => {
        this.loaded.set(rule);
        this.rule.set(draftOf(rule));
        this.conflict.set(false);
        this.serverCheck.set(null);
        this.status.set('ready');
      },
      error: (error: unknown) =>
        this.status.set(asApiError(error).status === 403 ? 'forbidden' : 'error'),
    });
  }

  protected setName(event: Event): void {
    const name = (event.target as HTMLInputElement).value;
    this.rule.update((r) => ({ ...r, name }));
  }

  protected setEnabled(enabled: boolean): void {
    this.rule.update((r) => ({ ...r, enabled }));
  }

  protected setRule(rule: AutomationRuleDraft): void {
    this.rule.set(rule);
    this.serverCheck.set(null);
  }

  protected async validate(): Promise<void> {
    const project = this.current.projectId();
    this.showProblems.set(true);
    if (!project || this.validating() || this.problems().length > 0) return;
    this.validating.set(true);
    this.serverError.set(null);
    try {
      this.serverCheck.set(await firstValueFrom(this.api.validate(project, this.rule())));
    } catch (error) {
      this.serverError.set(asApiError(error).message);
    } finally {
      this.validating.set(false);
    }
  }

  protected async save(): Promise<void> {
    const project = this.current.projectId();
    this.showProblems.set(true);
    if (!project || this.saving() || this.problems().length > 0) return;
    this.saving.set(true);
    this.serverError.set(null);
    try {
      const existing = this.loaded();
      const saved = existing
        ? await firstValueFrom(this.api.update(existing.id, existing.version, this.rule()))
        : await firstValueFrom(this.api.create(project, this.rule()));
      this.notifications.success(`La regla «${saved.name}» quedó guardada.`);
      await this.router.navigateByUrl(this.backLink());
    } catch (error) {
      const failure = asApiError(error);
      if (failure.code === 'automation.version_conflict' || failure.status === 409) {
        this.conflict.set(true);
      } else {
        this.serverError.set(failure.message);
      }
    } finally {
      this.saving.set(false);
    }
  }

  protected retryLoad(): void {
    const id = this.ruleId();
    if (id) this.loadRule(id);
  }

  protected reloadFromServer(): void {
    const existing = this.loaded();
    if (existing) this.loadRule(existing.id);
  }
}
