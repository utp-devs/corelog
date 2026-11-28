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
import { AgileSprint, CompleteSprintInput } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldRow } from '@corelog/ui/field-row';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';

import { issuesLabel, pointsLabel } from '../shared/planning-format';

export type CompleteDestinationKind = 'backlog' | 'sprint' | 'new';

/** Summary of the sprint scope the dialog shows before closing it. */
export interface CompleteSprintSummary {
  readonly doneIssues: number;
  readonly openIssues: number;
  readonly donePoints: number;
  readonly openPoints: number;
}

/**
 * Dialog to complete the active sprint: it summarises what is finished and what is not, and asks where the
 * unfinished issues go (backlog, another planned sprint or a new sprint). The owner runs the action.
 */
@Component({
  selector: 'app-complete-sprint-dialog',
  imports: [Button, Dialog, FieldRow, FormsModule, InputText, Message, SelectField],
  templateUrl: './complete-sprint-dialog.html',
  styleUrl: './complete-sprint-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CompleteSprintDialog {
  readonly open = input(false);
  readonly sprint = input<AgileSprint | null>(null);
  readonly summary = input<CompleteSprintSummary>({
    doneIssues: 0,
    openIssues: 0,
    donePoints: 0,
    openPoints: 0,
  });
  /** Planned sprints that can take the unfinished issues. */
  readonly plannedSprints = input<readonly AgileSprint[]>([]);
  readonly busy = input(false);
  readonly error = input<string | null>(null);

  readonly confirmed = output<CompleteSprintInput>();
  readonly closeRequested = output<void>();

  protected readonly kind = signal<CompleteDestinationKind>('backlog');
  protected readonly targetSprintId = signal<string | null>(null);
  protected readonly newName = signal('');

  protected readonly subtitle = computed(() => this.sprint()?.name);
  protected readonly doneText = computed(() => {
    const s = this.summary();
    return `${issuesLabel(s.doneIssues)} ${s.doneIssues === 1 ? 'terminada' : 'terminadas'} (${pointsLabel(s.donePoints)})`;
  });
  protected readonly openText = computed(() => {
    const s = this.summary();
    return `${issuesLabel(s.openIssues)} sin terminar (${pointsLabel(s.openPoints)})`;
  });
  protected readonly hasOpen = computed(() => this.summary().openIssues > 0);
  protected readonly sprintOptions = computed<SelectFieldOption[]>(() =>
    this.plannedSprints().map((sprint) => ({ value: sprint.id, label: sprint.name })),
  );
  protected readonly canPickSprint = computed(() => this.plannedSprints().length > 0);
  protected readonly invalid = computed(
    () => this.hasOpen() && this.kind() === 'sprint' && !this.targetSprintId(),
  );

  constructor() {
    effect(() => {
      if (!this.open()) return;
      untracked(() => {
        this.kind.set('backlog');
        this.targetSprintId.set(this.plannedSprints()[0]?.id ?? null);
        this.newName.set('');
      });
    });
  }

  protected setKind(kind: CompleteDestinationKind): void {
    this.kind.set(kind);
  }

  protected submit(): void {
    if (this.invalid()) return;
    const kind = this.hasOpen() ? this.kind() : 'backlog';
    if (kind === 'sprint') {
      this.confirmed.emit({ destination: `sprint:${this.targetSprintId()}` });
    } else if (kind === 'new') {
      const name = this.newName().trim();
      this.confirmed.emit({ destination: 'new', newSprintName: name || undefined });
    } else {
      this.confirmed.emit({ destination: 'backlog' });
    }
  }
}
