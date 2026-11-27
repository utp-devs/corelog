import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  DraftStatus,
  STANDARD_FIELD_OPTIONS,
  WorkflowRule,
  WorkflowTransition,
  fieldLabel,
} from '@corelog/core';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import { WorkflowRuleList } from './rule-list';

/** Change a panel edit asks the owner to store in the transition. */
export type TransitionPatch = Partial<Omit<WorkflowTransition, 'id'>>;

/**
 * Editor of one transition: name, statuses it leaves from, target status, required fields and the typed
 * builders for conditions, validators and post-functions. Read-only without edit rights.
 */
@Component({
  selector: 'app-workflow-transition-panel',
  imports: [Button, FormsModule, InputText, SelectField, WorkflowRuleList],
  templateUrl: './transition-panel.html',
  styleUrl: './transition-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowTransitionPanel {
  readonly transition = input.required<WorkflowTransition>();
  readonly statuses = input.required<readonly DraftStatus[]>();
  readonly resolutions = input<readonly SelectFieldOption[]>([]);
  readonly canEdit = input(false);
  /** Message of the server for this transition (path `transitions[i]...`). */
  readonly problems = input<readonly string[]>([]);

  readonly changed = output<TransitionPatch>();
  readonly closed = output<void>();

  protected readonly fieldInput = signal('');
  protected readonly fieldChoices = STANDARD_FIELD_OPTIONS;
  protected readonly fieldLabelOf = fieldLabel;

  protected readonly targetOptions = computed<SelectFieldOption[]>(() =>
    this.statuses().map((status) => ({ value: status.ref, label: status.name })),
  );
  protected readonly anySource = computed(() => this.transition().from.length === 0);

  protected rename(name: string): void {
    this.changed.emit({ name });
  }

  protected setTarget(ref: string | null): void {
    if (!ref) return;
    // A transition cannot leave from the status it goes to.
    this.changed.emit({
      to: ref,
      from: this.transition().from.filter((item) => item !== ref),
    });
  }

  protected setAnySource(any: boolean): void {
    if (any) {
      this.changed.emit({ from: [] });
      return;
    }
    const first = this.statuses().find((status) => status.ref !== this.transition().to);
    if (first) this.changed.emit({ from: [first.ref] });
  }

  protected toggleSource(ref: string, checked: boolean): void {
    const current = this.transition().from;
    const next = checked ? [...current, ref] : current.filter((item) => item !== ref);
    this.changed.emit({ from: next });
  }

  protected addField(): void {
    const name = this.fieldInput().trim();
    if (!name || this.transition().requiredFields.includes(name)) {
      this.fieldInput.set('');
      return;
    }
    this.changed.emit({ requiredFields: [...this.transition().requiredFields, name] });
    this.fieldInput.set('');
  }

  protected removeField(name: string): void {
    this.changed.emit({
      requiredFields: this.transition().requiredFields.filter((item) => item !== name),
    });
  }

  protected setRules(
    key: 'conditions' | 'validators' | 'postFunctions',
    rules: readonly WorkflowRule[],
  ): void {
    this.changed.emit({ [key]: rules });
  }
}
