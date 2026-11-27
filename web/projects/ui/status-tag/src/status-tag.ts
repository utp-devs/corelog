import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { ISSUE_STATUS_META, IssueStatus } from '@corelog/core';
import { Tone, ToneTag } from '@corelog/ui/tone-tag';

const TONE_BY_STATUS: Readonly<Record<IssueStatus, Tone>> = {
  open: 'blue',
  in_progress: 'gold',
  resolved: 'green',
  closed: 'neutral',
};

/** Issue status with its semantic color. */
@Component({
  selector: 'app-status-tag',
  imports: [ToneTag],
  templateUrl: './status-tag.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class StatusTag {
  readonly status = input.required<IssueStatus>();

  protected readonly label = computed(() => ISSUE_STATUS_META[this.status()].label);
  protected readonly tone = computed(() => TONE_BY_STATUS[this.status()]);
}
