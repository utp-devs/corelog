import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BacklogIssue, issuePath } from '@corelog/core';
import { CatalogIcon } from '@corelog/ui/catalog-icon';
import { Lozenge, LozengeCategory } from '@corelog/ui/lozenge';
import { UserAvatar } from '@corelog/ui/user-avatar';

import { hoursLabel } from '../shared/planning-format';

const CATEGORY: Readonly<Record<string, LozengeCategory>> = {
  todo: 'todo',
  in_progress: 'progress',
  done: 'done',
};

/** Compact view of the selected backlog issue, with a link to the full issue. */
@Component({
  selector: 'app-backlog-issue-panel',
  imports: [CatalogIcon, Lozenge, RouterLink, UserAvatar],
  templateUrl: './backlog-issue-panel.html',
  styleUrl: './backlog-issue-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BacklogIssuePanel {
  readonly issue = input.required<BacklogIssue>();
  readonly closed = output<void>();

  protected readonly href = computed(() => issuePath(this.issue().key));
  protected readonly category = computed(() => CATEGORY[this.issue().statusCategory] ?? 'todo');
  protected readonly estimate = computed(() =>
    hoursLabel(this.issue().originalEstimateSeconds ?? 0),
  );
}
