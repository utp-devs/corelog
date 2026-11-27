import { ChangeDetectionStrategy, Component, inject, input, output, signal } from '@angular/core';
import {
  ApiError,
  ApiErrorDetail,
  Issue,
  IssueStore,
  IssueTransition,
  NotificationService,
  asApiError,
  transitionErrorMessage,
} from '@corelog/core';

import { TransitionDialog, TransitionDialogValue } from './transition-dialog';

/**
 * Runs workflow transitions on one issue. `run` executes a transition at once when it needs nothing from the
 * person, or opens the dialog first when it does (required fields, comment, resolution or assignee). Server
 * refusals with details stay in the dialog next to the fields they name; any other failure is a toast.
 * Put one next to the control that starts the transition and call `run` on it.
 */
@Component({
  selector: 'app-transition-runner',
  imports: [TransitionDialog],
  templateUrl: './transition-runner.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TransitionRunner {
  private readonly store = inject(IssueStore);
  private readonly notify = inject(NotificationService);

  readonly issueId = input.required<string>();
  /** Id of the status the issue is in now; the server refuses the run when it moved in the meantime. */
  readonly expectedStatusId = input<string | undefined>(undefined);
  /** Key and summary of the issue, shown under the dialog title. */
  readonly context = input<string>();
  readonly currentAssigneeId = input<string | null>(null);

  /** The transition ran; carries the updated issue. */
  readonly finished = output<Issue>();
  /** The transition failed outside the dialog, or the dialog was left after a failure. */
  readonly failed = output<ApiError>();
  /** The person left the dialog without running the transition. */
  readonly cancelled = output<void>();
  /** The running state changed (true while a request is in flight). */
  readonly busyChange = output<boolean>();

  protected readonly pending = signal<IssueTransition | null>(null);
  protected readonly open = signal(false);
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly details = signal<readonly ApiErrorDetail[]>([]);

  /** Starts a transition: runs it directly or asks for what it needs first. */
  run(transition: IssueTransition): void {
    if (this.busy()) return;
    if (transition.needsScreen) {
      this.pending.set(transition);
      this.error.set(null);
      this.details.set([]);
      this.open.set(true);
      return;
    }
    this.execute(transition, {}, false);
  }

  protected confirm(value: TransitionDialogValue): void {
    const transition = this.pending();
    if (transition) this.execute(transition, value, true);
  }

  protected close(): void {
    if (this.busy()) return;
    this.open.set(false);
    this.pending.set(null);
    this.cancelled.emit();
  }

  private execute(
    transition: IssueTransition,
    value: Partial<TransitionDialogValue>,
    fromDialog: boolean,
  ): void {
    this.setBusy(true);
    this.error.set(null);
    this.details.set([]);
    const fields = value.fields && Object.keys(value.fields).length > 0 ? value.fields : undefined;
    this.store
      .transition(this.issueId(), {
        transitionId: transition.id,
        fields,
        comment: value.comment,
        resolutionId: value.resolutionId,
        assigneeId: value.assigneeId,
        expectedStatusId: this.expectedStatusId(),
      })
      .subscribe({
        next: (issue) => {
          this.setBusy(false);
          this.open.set(false);
          this.pending.set(null);
          this.notify.success(
            `Estado cambiado a «${issue.workflowStatus?.name ?? transition.to.name}»`,
          );
          this.finished.emit(issue);
        },
        error: (raw: unknown) => {
          this.setBusy(false);
          const error = asApiError(raw);
          const showInDialog =
            fromDialog && error.code === 'workflow.validation_failed' && error.details.length > 0;
          if (showInDialog) {
            this.details.set(error.details);
            return;
          }
          if (fromDialog) {
            this.error.set(transitionErrorMessage(error));
          } else {
            this.notify.error(transitionErrorMessage(error));
          }
          // A conflict or a refusal means the list of transitions the person sees is stale.
          if (!fromDialog || error.code !== 'workflow.validation_failed') this.failed.emit(error);
        },
      });
  }

  private setBusy(value: boolean): void {
    this.busy.set(value);
    this.busyChange.emit(value);
  }
}
