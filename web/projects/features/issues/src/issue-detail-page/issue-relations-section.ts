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
import { RouterLink } from '@angular/router';
import { FormsModule } from '@angular/forms';
import {
  CatalogStore,
  IssueApiService,
  IssueChildren,
  IssueLinkEntry,
  IssueLinkKindV2,
  IssueStatus,
  IssueViewApiService,
  IssueViewStore,
  LINK_KIND_OPTIONS,
  NotificationService,
  asApiError,
  issuePath,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldRow } from '@corelog/ui/field-row';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { Section, SectionActions } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';
import { StatusTag } from '@corelog/ui/status-tag';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

import { IssuePicker } from '../issue-picker/issue-picker';

interface LinkGroup {
  readonly label: string;
  readonly links: readonly IssueLinkEntry[];
}

const STATUSES: readonly string[] = ['open', 'in_progress', 'resolved', 'closed'];

/**
 * Child issues (table with progress and a quick "add subtask" row) and typed links of one issue,
 * grouped by the label seen from this issue ("bloquea", "bloqueada por"...). Adding a link opens a
 * dialog with the kind and an issue search.
 */
@Component({
  selector: 'app-issue-relations-section',
  imports: [
    AppTooltip,
    Button,
    Dialog,
    FieldRow,
    FormsModule,
    IssuePicker,
    LoadingSkeleton,
    Message,
    ProgressBar,
    RouterLink,
    Section,
    SectionActions,
    SelectField,
    StatusTag,
  ],
  templateUrl: './issue-relations-section.html',
  styleUrl: './issue-relations-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueRelationsSection {
  private readonly viewApi = inject(IssueViewApiService);
  private readonly issueApi = inject(IssueApiService);
  private readonly viewStore = inject(IssueViewStore);
  private readonly catalog = inject(CatalogStore);
  private readonly notify = inject(NotificationService);

  readonly issueId = input.required<string>();
  readonly projectId = input.required<string>();
  readonly archived = input(false);
  /** Hierarchy level of the issue: subtasks cannot have children. */
  readonly hierarchyLevel = input(0);
  /** Opens the link dialog from outside (the actions menu). */
  readonly linkRequest = input(0);

  protected readonly children = signal<IssueChildren | null>(null);
  protected readonly links = signal<readonly IssueLinkEntry[]>([]);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);

  protected readonly newSubtask = signal('');
  protected readonly adding = signal(false);

  protected readonly linkOpen = signal(false);
  protected readonly linkKind = signal<IssueLinkKindV2>('relates');
  protected readonly linkTarget = signal('');
  protected readonly linkSaving = signal(false);
  protected readonly linkError = signal<string | null>(null);
  protected readonly kindOptions = LINK_KIND_OPTIONS.map((o) => ({
    value: o.value,
    label: o.label,
  }));

  protected readonly canEdit = computed(() => !this.archived() && this.viewStore.can('edit_issue'));
  protected readonly canLink = computed(() => !this.archived() && this.viewStore.can('link_issue'));
  protected readonly canAddChild = computed(
    () => this.canEdit() && this.hierarchyLevel() !== -1 && this.subtaskType() !== undefined,
  );
  protected readonly subtaskType = computed(() =>
    this.catalog.issueTypes().find((type) => type.hierarchyLevel === -1),
  );
  protected readonly groups = computed<LinkGroup[]>(() => {
    const byLabel = new Map<string, IssueLinkEntry[]>();
    for (const link of this.links()) {
      const list = byLabel.get(link.label) ?? [];
      list.push(link);
      byLabel.set(link.label, list);
    }
    return Array.from(byLabel, ([label, links]) => ({ label, links }));
  });

  protected readonly pathOf = issuePath;

  constructor() {
    this.catalog.ensureLoaded();
    effect(() => {
      const id = this.issueId();
      untracked(() => void this.load(id));
    });
    effect(() => {
      if (this.linkRequest() > 0) untracked(() => this.openLink());
    });
  }

  protected statusOf(value: string): IssueStatus {
    return (STATUSES.includes(value) ? value : 'open') as IssueStatus;
  }

  protected openLink(): void {
    this.linkKind.set('relates');
    this.linkTarget.set('');
    this.linkError.set(null);
    this.linkOpen.set(true);
  }

  protected closeLink(): void {
    if (!this.linkSaving()) this.linkOpen.set(false);
  }

  protected async createLink(): Promise<void> {
    const target = this.linkTarget();
    if (!target) {
      this.linkError.set('Elige la incidencia que quieres vincular.');
      return;
    }
    this.linkSaving.set(true);
    this.linkError.set(null);
    try {
      await firstValueFrom(this.viewApi.createLink(this.issueId(), target, this.linkKind()));
      this.linkOpen.set(false);
      this.notify.success('Vínculo creado');
      await this.load(this.issueId(), false);
      void this.viewStore.refresh();
    } catch (error) {
      this.linkError.set(asApiError(error).message);
    } finally {
      this.linkSaving.set(false);
    }
  }

  protected async removeLink(link: IssueLinkEntry): Promise<void> {
    try {
      await firstValueFrom(this.viewApi.deleteLink(this.issueId(), link.id));
      this.links.update((list) => list.filter((item) => item.id !== link.id));
      this.viewStore.adjustCounts({ links: -1 });
      this.notify.success('Vínculo eliminado');
    } catch (error) {
      this.notify.error(asApiError(error).message);
    }
  }

  protected async addSubtask(): Promise<void> {
    const summary = this.newSubtask().trim();
    const type = this.subtaskType();
    if (!summary || !type) return;
    this.adding.set(true);
    try {
      await firstValueFrom(
        this.issueApi.create({
          projectId: this.projectId(),
          summary,
          description: summary,
          issueTypeId: type.id,
          parentId: this.issueId(),
        }),
      );
      this.newSubtask.set('');
      this.notify.success('Subtarea creada');
      await this.load(this.issueId(), false);
      void this.viewStore.refresh();
    } catch (error) {
      this.notify.error(asApiError(error).message);
    } finally {
      this.adding.set(false);
    }
  }

  private async load(issueId: string, skeleton = true): Promise<void> {
    if (skeleton) this.loading.set(true);
    this.error.set(null);
    try {
      const [children, links] = await Promise.all([
        firstValueFrom(this.viewApi.children(issueId)),
        firstValueFrom(this.viewApi.listLinks(issueId)),
      ]);
      if (issueId !== this.issueId()) return;
      this.children.set(children);
      this.links.set(links);
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.loading.set(false);
    }
  }
}
