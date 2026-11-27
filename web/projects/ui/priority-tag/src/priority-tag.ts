import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { ISSUE_PRIORITY_LABELS, IssuePriority } from '@corelog/core';

const ICON_BY_PRIORITY: Readonly<Record<IssuePriority, string>> = {
  low: 'pi pi-angle-down',
  medium: 'pi pi-equals',
  high: 'pi pi-angle-up',
  critical: 'pi pi-angle-double-up',
};

/** Issue priority: a colored icon followed by the label in regular text color. */
@Component({
  selector: 'app-priority-tag',
  templateUrl: './priority-tag.html',
  styleUrl: './priority-tag.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PriorityTag {
  readonly priority = input.required<IssuePriority>();

  protected readonly label = computed(() => ISSUE_PRIORITY_LABELS[this.priority()]);
  protected readonly icon = computed(() => ICON_BY_PRIORITY[this.priority()]);
}
