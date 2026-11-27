import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { ISSUE_PRIORITY_LABELS, IssuePriority } from '@corelog/core';
import { Tone, ToneTag } from '@corelog/ui/tone-tag';

const TONE_BY_PRIORITY: Readonly<Record<IssuePriority, Tone>> = {
  low: 'neutral',
  medium: 'blue',
  high: 'orange',
  critical: 'red',
};

/** Issue priority with its semantic color. */
@Component({
  selector: 'app-priority-tag',
  imports: [ToneTag],
  templateUrl: './priority-tag.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PriorityTag {
  readonly priority = input.required<IssuePriority>();

  protected readonly label = computed(() => ISSUE_PRIORITY_LABELS[this.priority()]);
  protected readonly tone = computed(() => TONE_BY_PRIORITY[this.priority()]);
}
