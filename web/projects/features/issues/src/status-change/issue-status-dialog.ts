import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import {
  BulkTransitionResult,
  Issue,
  IssueStore,
  IssueTransition,
  NotificationService,
  StatusCatalogStore,
  asApiError,
  transitionErrorMessage,
  workflowCategoryVisual,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { TransitionRunner } from '@corelog/ui/transition-dialog';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

interface Choice {
  /** Transition id, or status id when the issues do not share a source status. */
  readonly value: string;
  readonly label: string;
  readonly target: string;
  readonly category: 'todo' | 'progress' | 'done';
  readonly needsScreen: boolean;
  readonly transition: IssueTransition | null;
}

/**
 * Changes the status of one issue (the navigator's row action) or of several (the bulk action). One issue
 * lists its available transitions and runs the chosen one with the transition dialog when it asks for data.
 * Several issues that share a status run one transition for all of them; otherwise the person picks the
 * target status and each issue is moved when the workflow allows it. The result reports what failed.
 */
@Component({
  selector: 'app-issue-status-dialog',
  imports: [Button, Dialog, Message, TransitionRunner],
  templateUrl: './issue-status-dialog.html',
  styleUrl: './issue-status-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueStatusDialog {
  private readonly store = inject(IssueStore);
  private readonly catalog = inject(StatusCatalogStore);
  private readonly notify = inject(NotificationService);
  private readonly runner = viewChild(TransitionRunner);

  readonly open = input(false);
  readonly issues = input.required<readonly Issue[]>();

  readonly closeRequested = output<void>();
  /** Some issues changed status: the owner may refresh its list. */
  readonly changed = output<void>();

  protected readonly loading = signal(false);
  protected readonly loadError = signal<string | null>(null);
  protected readonly transitions = signal<readonly IssueTransition[]>([]);
  protected readonly choice = signal<string | null>(null);
  protected readonly busy = signal(false);
  protected readonly result = signal<BulkTransitionResult | null>(null);
  protected readonly error = signal<string | null>(null);

  protected readonly bulk = computed(() => this.issues().length > 1);
  protected readonly single = computed(() => this.issues()[0] ?? null);
  /** Every selected issue sits in the same status (only then a transition is meaningful for all). */
  protected readonly sharedStatus = computed(() => {
    const ids = new Set(this.issues().map((issue) => issue.workflowStatus?.id ?? issue.status));
    return ids.size === 1;
  });
  protected readonly byTransition = computed(() => !this.bulk() || this.sharedStatus());

  protected readonly choices = computed<Choice[]>(() => {
    if (this.byTransition()) {
      return this.transitions().map((transition) => ({
        value: transition.id,
        label: transition.name,
        target: transition.to.name,
        category: workflowCategoryVisual(transition.to.category),
        needsScreen: transition.needsScreen,
        transition,
      }));
    }
    return this.catalog.statuses().map((status) => ({
      value: status.id,
      label: status.name,
      target: status.name,
      category: workflowCategoryVisual(status.category),
      needsScreen: false,
      transition: null,
    }));
  });

  protected readonly heading = computed(() =>
    this.bulk() ? `Cambiar el estado de ${this.issues().length} incidencias` : 'Cambiar estado',
  );
  protected readonly subtitle = computed(() => {
    const issue = this.single();
    return issue && !this.bulk() ? `${issue.key} · ${issue.summary}` : undefined;
  });

  constructor() {
    effect(() => {
      if (!this.open()) return;
      untracked(() => this.prepare());
    });
  }

  protected select(value: string): void {
    this.choice.set(value);
  }

  protected failureText(item: { issue: string; message: string }): string {
    return `${item.issue}: ${item.message}`;
  }

  protected confirm(): void {
    const value = this.choice();
    const chosen = this.choices().find((item) => item.value === value);
    if (!chosen || this.busy()) return;
    if (!this.bulk()) {
      const issue = this.single();
      if (!issue || !chosen.transition) return;
      this.closeRequested.emit();
      this.runner()?.run(chosen.transition);
      return;
    }
    this.busy.set(true);
    this.error.set(null);
    const selector = this.byTransition()
      ? { transitionId: chosen.value }
      : { statusId: chosen.value };
    this.store
      .bulkTransition(
        this.issues().map((issue) => issue.id),
        selector,
      )
      .subscribe({
        next: (outcome) => {
          this.busy.set(false);
          this.result.set(outcome);
          if (outcome.succeeded.length > 0) this.changed.emit();
          if (outcome.failed.length === 0) {
            this.notify.success(
              outcome.succeeded.length === 1
                ? 'Se cambió el estado de 1 incidencia'
                : `Se cambió el estado de ${outcome.succeeded.length} incidencias`,
            );
            this.closeRequested.emit();
          }
        },
        error: (raw: unknown) => {
          this.busy.set(false);
          this.error.set(transitionErrorMessage(asApiError(raw)));
        },
      });
  }

  protected onFinished(): void {
    this.changed.emit();
  }

  private prepare(): void {
    this.choice.set(null);
    this.result.set(null);
    this.error.set(null);
    this.loadError.set(null);
    this.transitions.set([]);
    const first = this.issues()[0];
    if (!first) return;
    if (this.byTransition()) {
      this.loading.set(true);
      this.store.listTransitions(first.id).subscribe({
        next: (items) => {
          this.transitions.set(items);
          this.loading.set(false);
        },
        error: (raw: unknown) => {
          this.loading.set(false);
          this.loadError.set(transitionErrorMessage(asApiError(raw)));
        },
      });
    } else {
      this.catalog.ensureLoaded();
    }
  }
}
