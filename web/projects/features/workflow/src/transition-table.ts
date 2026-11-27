import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { DraftStatus, WorkflowTransition, ruleCounts, workflowCategoryVisual } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';

interface TransitionRow {
  readonly transition: WorkflowTransition;
  readonly from: readonly DraftStatus[];
  readonly to: DraftStatus | undefined;
  readonly counts: { conditions: number; validators: number; postFunctions: number };
}

/**
 * Transitions of the workflow as a table: name, the statuses it leaves from ("Cualquiera" when none are
 * listed), the status it goes to and how many conditions, validators and post-functions it has. It is also
 * the text alternative of the diagram.
 */
@Component({
  selector: 'app-workflow-transition-table',
  imports: [AppTooltip, Button],
  templateUrl: './transition-table.html',
  styleUrl: './transition-table.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowTransitionTable {
  readonly transitions = input.required<readonly WorkflowTransition[]>();
  readonly statuses = input.required<readonly DraftStatus[]>();
  readonly selectedId = input<string | null>(null);
  readonly canEdit = input(false);

  readonly selected = output<string>();
  readonly added = output<void>();
  readonly duplicated = output<string>();
  readonly removed = output<string>();

  protected readonly visual = workflowCategoryVisual;

  protected readonly rows = computed<TransitionRow[]>(() => {
    const byRef = new Map(this.statuses().map((status) => [status.ref, status] as const));
    return this.transitions().map((transition) => ({
      transition,
      from: transition.from
        .map((ref) => byRef.get(ref))
        .filter((status): status is DraftStatus => status !== undefined),
      to: byRef.get(transition.to),
      counts: ruleCounts(transition),
    }));
  });

  protected badge(count: number, singular: string, plural: string): string {
    return count === 1 ? `1 ${singular}` : `${count} ${plural}`;
  }
}
