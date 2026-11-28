import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  GadgetData,
  GadgetIssueRow,
  IssueListGadgetData,
  issuePath,
  priorityRefFromKey,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { IssueKey } from '@corelog/ui/issue-key';
import { PriorityTag } from '@corelog/ui/priority-tag';
import { ToneTag } from '@corelog/ui/tone-tag';

import {
  COLUMN_LABELS,
  categoryTone,
  formatDateTime,
  formatShortDay,
  isPastDay,
} from './gadget-format';

/** Table of issues for the assigned, created and filter gadgets, with the columns the gadget asks for. */
@Component({
  selector: 'app-gadget-issues',
  imports: [EmptyState, IssueKey, PriorityTag, RouterLink, ToneTag],
  templateUrl: './gadget-issues.html',
  styleUrl: './gadget-issues.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GadgetIssues {
  readonly content = input.required<GadgetData>();
  readonly title = input('Incidencias');

  protected readonly data = computed<IssueListGadgetData | null>(() => {
    const content = this.content();
    return content.type === 'assigned_to_me' ||
      content.type === 'created_by_me' ||
      content.type === 'filter_results'
      ? content.data
      : null;
  });

  protected readonly columns = computed(() => {
    const data = this.data();
    const wanted = data && data.columns.length > 0 ? data.columns : ['key', 'summary', 'status'];
    return wanted.map((key) => ({ key, label: COLUMN_LABELS[key] ?? key }));
  });

  protected readonly emptyText = computed(() =>
    this.content().type === 'created_by_me'
      ? 'Todavía no has creado incidencias.'
      : this.content().type === 'filter_results'
        ? 'El filtro no devuelve incidencias.'
        : 'No tienes incidencias abiertas.',
  );

  protected link(row: GadgetIssueRow): string {
    return issuePath(row.key);
  }

  protected tone(row: GadgetIssueRow) {
    return categoryTone(row.statusCategory);
  }

  protected priority(row: GadgetIssueRow) {
    return priorityRefFromKey(row.priority);
  }

  protected due(row: GadgetIssueRow): string {
    return formatShortDay(row.due);
  }

  protected overdue(row: GadgetIssueRow): boolean {
    return row.statusCategory !== 'done' && isPastDay(row.due);
  }

  protected updated(row: GadgetIssueRow): string {
    return formatDateTime(row.updated);
  }
}
