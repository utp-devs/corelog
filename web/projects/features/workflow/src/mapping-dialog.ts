import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { WorkflowMappingRequirement } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';

/**
 * Asks where the issues of the statuses that leave the workflow go. Every removed status that holds issues
 * needs a target; the dialog does not confirm until each one has it.
 */
@Component({
  selector: 'app-workflow-mapping-dialog',
  imports: [Button, Dialog, FormsModule, SelectField],
  templateUrl: './mapping-dialog.html',
  styleUrl: './mapping-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowMappingDialog {
  readonly open = input(false);
  readonly requirements = input.required<readonly WorkflowMappingRequirement[]>();
  /** Saved statuses that stay in the workflow and can receive issues. */
  readonly targets = input.required<readonly SelectFieldOption[]>();
  readonly busy = input(false);
  readonly confirmLabel = input('Continuar');

  readonly confirmed = output<Record<string, string>>();
  readonly closeRequested = output<void>();

  protected readonly choices = signal<Readonly<Record<string, string>>>({});
  protected readonly complete = computed(() =>
    this.requirements().every((item) => !!this.choices()[item.statusId]),
  );

  constructor() {
    effect(() => {
      if (this.open()) untracked(() => this.choices.set({}));
    });
  }

  protected choose(statusId: string, target: string | null): void {
    this.choices.update((current) => {
      const next = { ...current };
      if (target) next[statusId] = target;
      else delete next[statusId];
      return next;
    });
  }

  protected choiceOf(statusId: string): string | null {
    return this.choices()[statusId] ?? null;
  }

  protected countText(count: number): string {
    return count === 1 ? '1 incidencia' : `${count} incidencias`;
  }

  protected confirm(): void {
    if (this.complete() && !this.busy()) this.confirmed.emit({ ...this.choices() });
  }
}
