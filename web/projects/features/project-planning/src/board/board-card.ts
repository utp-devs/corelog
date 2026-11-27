import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BoardCard, ISSUE_TYPES, IssueType, issuePath } from '@corelog/core';
import { Avatar } from '@corelog/ui/avatar';
import { IssueKey } from '@corelog/ui/issue-key';
import { Lozenge } from '@corelog/ui/lozenge';
import { PriorityTag } from '@corelog/ui/priority-tag';

/** Visual card of a board column: key, summary link, priority, points and assignee. */
@Component({
  selector: 'app-board-card',
  imports: [RouterLink, Avatar, IssueKey, Lozenge, PriorityTag],
  templateUrl: './board-card.html',
  styleUrl: './board-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BoardCardView {
  readonly card = input.required<BoardCard>();
  /** Renders the drag preview: the card is shown as it is carried. */
  readonly dragging = input(false);

  protected readonly shortId = computed(() => this.card().id.slice(0, 8));
  protected readonly href = computed(() => issuePath(this.card().id));
  protected readonly issueType = computed<IssueType | undefined>(() =>
    ISSUE_TYPES.find((type) => type === this.card().type),
  );
}
