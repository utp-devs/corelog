import {
  CdkDrag,
  CdkDragDrop,
  CdkDragPlaceholder,
  CdkDragPreview,
  CdkDropList,
} from '@angular/cdk/drag-drop';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import {
  BacklogAnchor,
  BacklogIssue,
  BacklogSection,
  SelectionMode,
  anchorAt,
  issuePath,
} from '@corelog/core';
import { DropdownMenu } from '@corelog/ui/app-shell';
import { CatalogIcon } from '@corelog/ui/catalog-icon';
import { Lozenge, LozengeCategory } from '@corelog/ui/lozenge';
import { AppTooltip } from '@corelog/ui/tooltip';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import {
  SPRINT_STATUS_LABELS,
  SPRINT_STATUS_TONES,
  categoryShares,
  hoursLabel,
  issuesLabel,
  pointsLabel,
  sprintDatesOf,
} from '../shared/planning-format';

/** A section the issues of a row can be sent to from its menu. */
export interface BacklogDestination {
  /** Sprint id; null is the backlog. */
  readonly sprintId: string | null;
  readonly name: string;
}

/** The issues to move and where, as the section asks the page. */
export interface BacklogRankRequest {
  readonly issueIds: readonly string[];
  readonly sprintId: string | null;
  readonly anchor: BacklogAnchor;
}

const CATEGORY_LOZENGE: Readonly<Record<string, LozengeCategory>> = {
  todo: 'todo',
  in_progress: 'progress',
  done: 'done',
};

const CATEGORY_NAMES: Readonly<Record<string, string>> = {
  todo: 'Por hacer',
  in_progress: 'En curso',
  done: 'Hecho',
};

const MAX_LABELS = 2;
const MIN_SUMMARY = 3;

/**
 * One section of the backlog (the active sprint, a planned sprint or the backlog): a header with the sprint
 * data and totals, and the ranked list of issues. Rows select with click, Ctrl and Shift, move by dragging
 * (all selected rows go together) or with the "Mover a" menu and Alt+Arrow keys, and edit points in place.
 */
@Component({
  selector: 'app-backlog-section',
  imports: [
    AppTooltip,
    Button,
    CatalogIcon,
    CdkDrag,
    CdkDragPlaceholder,
    CdkDragPreview,
    CdkDropList,
    DropdownMenu,
    FormsModule,
    InputText,
    Lozenge,
    RouterLink,
    UserAvatar,
  ],
  templateUrl: './backlog-section.html',
  styleUrl: './backlog-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BacklogSectionView {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly section = input.required<BacklogSection>();
  readonly selectedIds = input<readonly string[]>([]);
  /** Every issue id of the backlog in display order; it orders a multi-row move. */
  readonly orderedIds = input<readonly string[]>([]);
  readonly destinations = input<readonly BacklogDestination[]>([]);
  readonly canMove = input(false);
  readonly canManage = input(false);
  readonly readOnly = input(false);
  /** An active sprint is running, so a planned sprint cannot start. */
  readonly hasActive = input(false);

  readonly selectRequested = output<{ id: string; mode: SelectionMode }>();
  readonly rankRequested = output<BacklogRankRequest>();
  readonly startRequested = output<void>();
  readonly completeRequested = output<void>();
  readonly editRequested = output<void>();
  readonly deleteRequested = output<void>();
  readonly pointsChanged = output<{ id: string; points: number | null }>();
  readonly createRequested = output<string>();

  protected readonly collapsed = signal(false);
  protected readonly editingPoints = signal<string | null>(null);
  protected readonly pointsDraft = signal<number | null>(null);
  protected readonly creating = signal(false);
  protected readonly draft = signal('');
  protected readonly createError = signal<string | null>(null);

  protected readonly sprint = computed(() => this.section().sprint);
  protected readonly title = computed(() => this.sprint()?.name ?? 'Backlog');
  protected readonly isBacklog = computed(() => this.section().kind === 'backlog');
  protected readonly statusLabel = computed(() => {
    const sprint = this.sprint();
    return sprint ? SPRINT_STATUS_LABELS[sprint.status] : '';
  });
  protected readonly statusTone = computed(() => {
    const sprint = this.sprint();
    return sprint ? SPRINT_STATUS_TONES[sprint.status] : 'neutral';
  });
  protected readonly dates = computed(() => {
    const sprint = this.sprint();
    return sprint && (sprint.startDate || sprint.endDate) ? sprintDatesOf(sprint) : '';
  });
  protected readonly countText = computed(() => issuesLabel(this.section().totals.issues));
  protected readonly pointsText = computed(() => pointsLabel(this.section().totals.points));
  protected readonly shares = computed(() => categoryShares(this.section().totals));
  protected readonly categories = computed(() => {
    const by = this.section().totals.byCategory;
    return (['todo', 'in_progress', 'done'] as const).map((category) => ({
      category,
      label: CATEGORY_NAMES[category],
      lozenge: CATEGORY_LOZENGE[category],
      points: by[category]?.points ?? 0,
      issues: by[category]?.issues ?? 0,
    }));
  });
  protected readonly canStart = computed(
    () => this.section().kind === 'planned' && this.canManage() && !this.hasActive(),
  );
  protected readonly canComplete = computed(
    () => this.section().kind === 'active' && this.canManage(),
  );
  protected readonly hasMenu = computed(
    () => this.sprint() !== null && this.canManage() && this.sprint()?.status !== 'closed',
  );
  protected readonly canDelete = computed(() => this.section().kind === 'planned');
  protected readonly dropDisabled = computed(() => this.readOnly() || !this.canMove());
  protected readonly selected = computed(() => new Set(this.selectedIds()));

  protected hrefOf(issue: BacklogIssue): string {
    return issuePath(issue.key);
  }

  protected lozengeOf(issue: BacklogIssue): LozengeCategory {
    return CATEGORY_LOZENGE[issue.statusCategory] ?? 'todo';
  }

  protected visibleLabels(issue: BacklogIssue): readonly string[] {
    return issue.labels.slice(0, MAX_LABELS).map((label) => label.name);
  }

  protected hiddenLabels(issue: BacklogIssue): number {
    return Math.max(0, issue.labels.length - MAX_LABELS);
  }

  protected hoursOf(issue: BacklogIssue): string {
    return hoursLabel(issue.originalEstimateSeconds ?? 0);
  }

  /** Destinations other than this section, for the row menu. */
  protected otherDestinations(): readonly BacklogDestination[] {
    const own = this.sprint()?.id ?? null;
    return this.destinations().filter((destination) => destination.sprintId !== own);
  }

  /* Selection */

  protected onRowClick(event: MouseEvent, issue: BacklogIssue): void {
    const target = event.target as HTMLElement;
    if (target.closest('a, button, input, [role="menuitem"]')) return;
    const mode: SelectionMode = event.shiftKey
      ? 'range'
      : event.ctrlKey || event.metaKey
        ? 'toggle'
        : 'single';
    this.selectRequested.emit({ id: issue.id, mode });
  }

  protected onRowKeydown(event: KeyboardEvent, issue: BacklogIssue, index: number): void {
    if (event.target !== event.currentTarget) return;
    if (event.key === ' ' || event.key === 'Enter') {
      event.preventDefault();
      this.selectRequested.emit({ id: issue.id, mode: event.ctrlKey ? 'toggle' : 'single' });
      return;
    }
    if (!event.altKey || this.dropDisabled()) return;
    if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
      event.preventDefault();
      this.nudge(issue, index, event.key === 'ArrowUp' ? -1 : 1);
    }
  }

  /* Moves */

  protected onDrop(event: CdkDragDrop<BacklogSection>): void {
    const issue = event.item.data as BacklogIssue;
    const target = event.container.data;
    if (this.dropDisabled()) return;
    const moving = this.movingFor(issue);
    const movingSet = new Set(moving);
    const ids = target.issues.map((item) => item.id).filter((id) => id !== issue.id);
    const sameSpot =
      event.previousContainer === event.container &&
      event.previousIndex === event.currentIndex &&
      moving.length === 1;
    if (sameSpot) return;
    this.rankRequested.emit({
      issueIds: moving,
      sprintId: target.sprint?.id ?? null,
      anchor: anchorAt(ids, event.currentIndex, movingSet),
    });
  }

  protected moveTo(issue: BacklogIssue, destination: BacklogDestination): void {
    this.rankRequested.emit({
      issueIds: this.movingFor(issue),
      sprintId: destination.sprintId,
      anchor: {},
    });
  }

  protected moveToTop(issue: BacklogIssue): void {
    const first = this.section().issues.find((item) => item.id !== issue.id);
    this.rankRequested.emit({
      issueIds: this.movingFor(issue),
      sprintId: this.sprint()?.id ?? null,
      anchor: first ? { beforeIssueId: first.id } : {},
    });
  }

  private nudge(issue: BacklogIssue, index: number, step: -1 | 1): void {
    const issues = this.section().issues;
    const neighbor = issues[index + step];
    if (!neighbor) return;
    this.rankRequested.emit({
      issueIds: [issue.id],
      sprintId: this.sprint()?.id ?? null,
      anchor: step === -1 ? { beforeIssueId: neighbor.id } : { afterIssueId: neighbor.id },
    });
  }

  /** The dragged issue moves alone, or with every selected row when it is one of them. */
  protected movingFor(issue: BacklogIssue): readonly string[] {
    const selected = this.selected();
    if (!selected.has(issue.id) || selected.size < 2) return [issue.id];
    const order = this.orderedIds();
    return order.filter((id) => selected.has(id));
  }

  /* Points */

  protected startPoints(issue: BacklogIssue): void {
    if (!this.canMove() || this.readOnly()) return;
    this.pointsDraft.set(issue.storyPoints);
    this.editingPoints.set(issue.id);
  }

  protected savePoints(issue: BacklogIssue): void {
    const value = this.pointsDraft();
    this.editingPoints.set(null);
    const points = value === null || (value as unknown) === '' ? null : Number(value);
    if (points === issue.storyPoints || (points !== null && (Number.isNaN(points) || points < 0)))
      return;
    this.pointsChanged.emit({ id: issue.id, points });
  }

  protected cancelPoints(): void {
    this.editingPoints.set(null);
  }

  /* Inline create */

  protected startCreate(): void {
    this.creating.set(true);
    this.draft.set('');
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLInputElement>('[data-testid="backlog-create-input"]')
          ?.focus(),
      { injector: this.injector },
    );
  }

  protected cancelCreate(): void {
    this.creating.set(false);
  }

  protected submitCreate(): void {
    const summary = this.draft().trim();
    if (summary.length < MIN_SUMMARY) {
      this.createError.set('Escribe al menos 3 caracteres.');
      return;
    }
    this.createError.set(null);
    this.createRequested.emit(summary);
    this.draft.set('');
  }
}
