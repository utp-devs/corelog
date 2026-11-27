import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { ISSUE_TYPE_LABELS, IssueType } from '@corelog/core';

const ICON_CLASS: Readonly<Record<IssueType, string>> = {
  task: 'pi pi-check-square',
  bug: '',
  story: 'pi pi-book',
  epic: 'pi pi-bolt',
  subtask: 'pi pi-sitemap',
};

/** Icon with its own color per issue type. The type name stays available to screen readers. */
@Component({
  selector: 'app-issue-type-icon',
  templateUrl: './issue-type-icon.html',
  styleUrl: './issue-type-icon.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueTypeIcon {
  readonly type = input.required<IssueType>();
  /** Shows the type name next to the icon. */
  readonly showLabel = input(false);

  protected readonly label = computed(() => ISSUE_TYPE_LABELS[this.type()]);
  protected readonly iconClass = computed(() => ICON_CLASS[this.type()]);
}
