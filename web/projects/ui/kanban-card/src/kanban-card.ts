import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BoardCardField, BoardViewCard, issuePath } from '@corelog/core';
import { DropdownMenu } from '@corelog/ui/app-shell';
import { CatalogIcon } from '@corelog/ui/catalog-icon';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { AppTooltip } from '@corelog/ui/tooltip';

/** All the fields a card can show, in the order it shows them. */
export const KANBAN_CARD_ALL_FIELDS: readonly BoardCardField[] = [
  'key',
  'type',
  'priority',
  'assignee',
  'labels',
  'points',
  'epic',
  'subtasks',
  'flag',
];

/** A column the card can be moved to from its menu. */
export interface KanbanMoveTarget {
  readonly id: string;
  readonly name: string;
}

const MAX_LABELS = 2;

/**
 * Card of a board column: type, key, summary, priority, assignee, labels, points, epic, subtasks and flag.
 * The summary is a link to the issue that covers the whole card. Editors also get a "Mover a" menu and the
 * Alt+Arrow shortcuts, the keyboard alternative to dragging.
 */
@Component({
  selector: 'app-kanban-card',
  imports: [RouterLink, AppTooltip, CatalogIcon, DropdownMenu, UserAvatar],
  templateUrl: './kanban-card.html',
  styleUrl: './kanban-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class KanbanCard {
  readonly card = input.required<BoardViewCard>();
  /** Fields to show; an empty list hides everything but the summary. */
  readonly fields = input<readonly BoardCardField[]>(KANBAN_CARD_ALL_FIELDS);
  /** Columns offered by the "Mover a" menu. Without targets the menu is hidden. */
  readonly targets = input<readonly KanbanMoveTarget[]>([]);
  /** Draws the card as the carried preview. */
  readonly dragging = input(false);
  /** The card is the dimmed original left behind while it is carried. */
  readonly placeholder = input(false);
  /** Emits the id of the column chosen in the menu. */
  readonly moved = output<string>();
  /** Alt+ArrowLeft (-1) or Alt+ArrowRight (1): move to the neighbor column. */
  readonly shifted = output<-1 | 1>();

  protected readonly href = computed(() => issuePath(this.card().key));
  protected readonly shown = computed(() => new Set(this.fields()));
  protected readonly visibleLabels = computed(() => this.card().labels.slice(0, MAX_LABELS));
  protected readonly hiddenLabels = computed(() =>
    Math.max(0, this.card().labels.length - MAX_LABELS),
  );
  protected readonly hasMeta = computed(() => {
    const card = this.card();
    const shown = this.shown();
    return (
      (shown.has('epic') && card.epic !== null) ||
      (shown.has('labels') && card.labels.length > 0) ||
      (shown.has('subtasks') && card.subtasks.total > 0)
    );
  });

  protected onKeydown(event: KeyboardEvent): void {
    if (!event.altKey || this.targets().length === 0) return;
    if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
      event.preventDefault();
      this.shifted.emit(event.key === 'ArrowRight' ? 1 : -1);
    }
  }
}
