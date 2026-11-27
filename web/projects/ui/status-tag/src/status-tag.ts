import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import {
  ISSUE_STATUS_META,
  IssueStatus,
  WorkflowStatusRef,
  workflowCategoryVisual,
} from '@corelog/core';
import { Tone, ToneTag } from '@corelog/ui/tone-tag';

const TONE_BY_STATUS: Readonly<Record<IssueStatus, Tone>> = {
  open: 'neutral',
  in_progress: 'blue',
  resolved: 'green',
  closed: 'neutral',
};

const TONE_BY_CATEGORY: Readonly<Record<'todo' | 'progress' | 'done', Tone>> = {
  todo: 'neutral',
  progress: 'blue',
  done: 'green',
};

/**
 * Issue status with its semantic color. The workflow status (name and category) wins when it is given;
 * the legacy status is the fallback for payloads that do not carry it.
 */
@Component({
  selector: 'app-status-tag',
  imports: [ToneTag],
  templateUrl: './status-tag.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class StatusTag {
  readonly status = input.required<IssueStatus>();
  readonly workflowStatus = input<Pick<WorkflowStatusRef, 'name' | 'category'> | null | undefined>(
    null,
  );

  protected readonly label = computed(
    () => this.workflowStatus()?.name ?? ISSUE_STATUS_META[this.status()].label,
  );
  protected readonly tone = computed(() => {
    const workflow = this.workflowStatus();
    return workflow
      ? TONE_BY_CATEGORY[workflowCategoryVisual(workflow.category)]
      : TONE_BY_STATUS[this.status()];
  });
}
