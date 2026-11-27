import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  BoardCard,
  BoardMoveTarget,
  ISSUE_TYPES,
  IssueType,
  issueKeyOf,
  issuePath,
} from '@corelog/core';
import { DropdownMenu } from '@corelog/ui/app-shell';
import { Avatar } from '@corelog/ui/avatar';
import { IssueKey } from '@corelog/ui/issue-key';
import { Lozenge } from '@corelog/ui/lozenge';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { AppTooltip } from '@corelog/ui/tooltip';

import { shortKeyOf } from '../shared/work-labels';

/**
 * Visual card of a board column: type icon and key, title (two lines), priority, points and assignee.
 * Editors also get a "Mover a" menu, the keyboard and touch alternative to dragging.
 */
@Component({
  selector: 'app-board-card',
  imports: [RouterLink, AppTooltip, Avatar, DropdownMenu, IssueKey, Lozenge, PriorityTag],
  templateUrl: './board-card.html',
  styleUrl: './board-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BoardCardView {
  readonly card = input.required<BoardCard>();
  /** Renders the drag preview: the card is shown as it is carried. */
  readonly dragging = input(false);
  /** Columns the card can move to. With no targets, or in read-only mode, the menu is not shown. */
  readonly targets = input<readonly BoardMoveTarget[]>([]);
  readonly readOnly = input(false);
  /** Emits the id of the column picked in the "Mover a" menu. */
  readonly moved = output<string>();

  protected readonly fullKey = computed(() => issueKeyOf(this.card().id));
  protected readonly shortKey = computed(() => shortKeyOf(this.card().id));
  protected readonly href = computed(() => issuePath(this.card().id));
  protected readonly issueType = computed<IssueType | undefined>(() =>
    ISSUE_TYPES.find((type) => type === this.card().type),
  );
  protected readonly showMenu = computed(() => !this.readOnly() && this.targets().length > 0);
}
