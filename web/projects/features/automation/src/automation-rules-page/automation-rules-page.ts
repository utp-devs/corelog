import { DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  AutomationApiService,
  CurrentProjectStore,
  NotificationService,
  asApiError,
  projectPath,
  type AutomationRule,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { duplicateDraft, summarizeTrigger } from '@corelog/ui/rule-builder';
import { Section, SectionActions } from '@corelog/ui/section';
import { FormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { firstValueFrom } from 'rxjs';

import { RuleRunsDrawer } from '../rule-runs-drawer/rule-runs-drawer';

type LoadStatus = 'loading' | 'ready' | 'error' | 'forbidden';

/**
 * Automation rules of the project: what each one does in plain words, a switch to turn it on or off, how often
 * it ran, and the actions to edit, duplicate, read its history or delete it. Needs the permission to manage
 * automation; an archived project is read-only.
 */
@Component({
  selector: 'app-automation-rules-page',
  imports: [
    Button,
    FormsModule,
    ConfirmModal,
    DatePipe,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    RouterLink,
    RuleRunsDrawer,
    Section,
    SectionActions,
    ToggleSwitch,
  ],
  templateUrl: './automation-rules-page.html',
  styleUrl: './automation-rules-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AutomationRulesPage {
  private readonly api = inject(AutomationApiService);
  private readonly current = inject(CurrentProjectStore);
  private readonly notifications = inject(NotificationService);

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly rules = signal<readonly AutomationRule[]>([]);
  protected readonly runsOf = signal<AutomationRule | null>(null);
  protected readonly deleting = signal<AutomationRule | null>(null);
  protected readonly busy = signal(false);

  protected readonly archived = this.current.isArchived;
  protected readonly canManage = computed(() => this.current.can('manage_automation'));
  protected readonly canEdit = computed(() => this.canManage() && !this.archived());
  protected readonly newLink = computed(() => this.link('new'));

  constructor() {
    effect(() => {
      const id = this.current.projectId();
      const allowed = this.current.permissionsLoaded() ? this.canManage() : null;
      untracked(() => {
        if (!id || allowed === null) return;
        if (!allowed) this.status.set('forbidden');
        else this.load();
      });
    });
  }

  protected link(path: string): string {
    const project = this.current.project();
    return project ? projectPath(project, `settings/automation/${path}`) : '.';
  }

  protected trigger(rule: AutomationRule): string {
    const text = summarizeTrigger(rule.trigger);
    return text.charAt(0).toUpperCase() + text.slice(1);
  }

  protected load(): void {
    const id = this.current.projectId();
    if (!id) return;
    this.status.set('loading');
    this.api.list(id).subscribe({
      next: (rules) => {
        this.rules.set(rules);
        this.status.set('ready');
      },
      error: (error: unknown) =>
        this.status.set(asApiError(error).status === 403 ? 'forbidden' : 'error'),
    });
  }

  protected async setEnabled(rule: AutomationRule, enabled: boolean): Promise<void> {
    try {
      const updated = await firstValueFrom(this.api.setEnabled(rule.id, enabled));
      this.rules.update((list) => list.map((r) => (r.id === rule.id ? updated : r)));
    } catch (error) {
      this.notifications.error(asApiError(error));
      this.rules.update((list) => list.map((r) => (r.id === rule.id ? { ...rule } : r)));
    }
  }

  protected async duplicate(rule: AutomationRule): Promise<void> {
    const project = this.current.projectId();
    if (!project || this.busy()) return;
    this.busy.set(true);
    try {
      const copy = await firstValueFrom(this.api.create(project, duplicateDraft(rule)));
      this.rules.update((list) => [copy, ...list]);
      this.notifications.success(`Se creó «${copy.name}», desactivada.`);
    } catch (error) {
      this.notifications.error(asApiError(error));
    } finally {
      this.busy.set(false);
    }
  }

  protected async remove(): Promise<void> {
    const rule = this.deleting();
    if (!rule || this.busy()) return;
    this.busy.set(true);
    try {
      await firstValueFrom(this.api.remove(rule.id));
      this.rules.update((list) => list.filter((r) => r.id !== rule.id));
      this.notifications.success(`Se eliminó la regla «${rule.name}».`);
      this.deleting.set(null);
    } catch (error) {
      this.notifications.error(asApiError(error));
    } finally {
      this.busy.set(false);
    }
  }
}
