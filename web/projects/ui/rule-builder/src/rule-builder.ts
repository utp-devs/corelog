import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import type {
  AutomationAction,
  AutomationActionType,
  AutomationCondition,
  AutomationConditionType,
  AutomationRuleDraft,
  AutomationTrigger,
  AutomationTriggerType,
} from '@corelog/core';
import { CoreqlField } from '@corelog/ui/coreql-field';
import { AppTooltip } from '@corelog/ui/tooltip';

import {
  ACTION_CHOICES,
  ASSIGN_CHOICES,
  COMPARABLE_FIELDS,
  CONDITION_CHOICES,
  MAX_ACTIONS,
  MAX_CONDITIONS,
  NOTIFY_CHOICES,
  ROLE_CHOICES,
  SETTABLE_FIELDS,
  SMART_VALUES,
  TRIGGER_CHOICES,
  UPDATED_FIELDS,
  actionLabel,
  actionOfType,
  addItem,
  conditionLabel,
  conditionOfType,
  moveItem,
  parseList,
  problemsOf,
  removeAt,
  replaceAt,
  summarizeRule,
  triggerOfType,
  type RuleProblem,
} from './rule-logic';

/** A person or team an action can point at. */
export interface RuleTarget {
  readonly id: string;
  readonly name: string;
}

/**
 * Editor of the definition of an automation rule: the trigger with its parameters, an ordered list of conditions
 * and an ordered list of actions with keyboard-accessible move buttons, smart-value chips for texts and a live
 * sentence that reads the rule back. It edits the `rule` model; name and saving belong to the page.
 */
@Component({
  selector: 'app-rule-builder',
  imports: [AppTooltip, CoreqlField],
  templateUrl: './rule-builder.html',
  styleUrl: './rule-builder.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RuleBuilder {
  readonly rule = model.required<AutomationRuleDraft>();
  /** Status names offered when a field asks for a status. */
  readonly statuses = input<readonly string[]>([]);
  readonly people = input<readonly RuleTarget[]>([]);
  readonly teams = input<readonly RuleTarget[]>([]);
  readonly problems = input<readonly RuleProblem[]>([]);
  readonly readonly = input(false);

  protected readonly triggerChoices = TRIGGER_CHOICES;
  protected readonly conditionChoices = CONDITION_CHOICES;
  protected readonly actionChoices = ACTION_CHOICES;
  protected readonly comparable = COMPARABLE_FIELDS;
  protected readonly settable = SETTABLE_FIELDS;
  protected readonly updatedFields = UPDATED_FIELDS;
  protected readonly roles = ROLE_CHOICES;
  protected readonly assignChoices = ASSIGN_CHOICES;
  protected readonly notifyChoices = NOTIFY_CHOICES;
  protected readonly smartValues = SMART_VALUES;
  protected readonly maxActions = MAX_ACTIONS;
  protected readonly maxConditions = MAX_CONDITIONS;
  protected readonly conditionLabel = conditionLabel;
  protected readonly actionLabel = actionLabel;

  protected readonly summary = computed(() => summarizeRule(this.rule(), (id) => this.nameOf(id)));
  protected readonly trigger = computed(() => this.rule().trigger);

  /** Text control that last had the focus, where a smart value chip lands. */
  private lastText: { el: HTMLTextAreaElement | HTMLInputElement; index: number } | null = null;

  protected issues(path: string): readonly string[] {
    return problemsOf(this.problems(), path);
  }

  private nameOf(id: string): string {
    return (
      this.people().find((p) => p.id === id)?.name ??
      this.teams().find((t) => t.id === id)?.name ??
      '…'
    );
  }

  private value(event: Event): string {
    return (event.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value;
  }

  // Trigger.

  protected setTriggerType(event: Event): void {
    this.rule.update((r) => ({
      ...r,
      trigger: triggerOfType(this.value(event) as AutomationTriggerType),
    }));
  }

  protected patchTrigger(patch: Partial<AutomationTrigger>): void {
    this.rule.update((r) => ({ ...r, trigger: { ...r.trigger, ...patch } }));
  }

  protected setEvery(event: Event): void {
    const raw = this.value(event);
    this.patchTrigger({ everyMinutes: raw === '' ? undefined : Number(raw) });
  }

  // Conditions.

  protected addCondition(): void {
    this.rule.update((r) => ({
      ...r,
      conditions: addItem(r.conditions, conditionOfType('field_equals'), MAX_CONDITIONS),
    }));
  }

  protected setConditionType(index: number, event: Event): void {
    const next = conditionOfType(this.value(event) as AutomationConditionType);
    this.rule.update((r) => ({ ...r, conditions: replaceAt(r.conditions, index, next) }));
  }

  protected patchCondition(index: number, patch: Partial<AutomationCondition>): void {
    this.rule.update((r) => ({
      ...r,
      conditions: replaceAt(r.conditions, index, { ...r.conditions[index], ...patch }),
    }));
  }

  protected setConditionValues(index: number, event: Event): void {
    this.patchCondition(index, { values: parseList(this.value(event)) });
  }

  protected listText(values: readonly string[] | undefined): string {
    return (values ?? []).join(', ');
  }

  protected removeCondition(index: number): void {
    this.rule.update((r) => ({ ...r, conditions: removeAt(r.conditions, index) }));
  }

  protected moveCondition(index: number, delta: -1 | 1): void {
    this.rule.update((r) => ({ ...r, conditions: moveItem(r.conditions, index, delta) }));
  }

  // Actions.

  protected addAction(): void {
    this.rule.update((r) => ({
      ...r,
      actions: addItem(r.actions, actionOfType('add_comment'), MAX_ACTIONS),
    }));
  }

  protected setActionType(index: number, event: Event): void {
    const next = actionOfType(this.value(event) as AutomationActionType);
    this.rule.update((r) => ({ ...r, actions: replaceAt(r.actions, index, next) }));
  }

  protected patchAction(index: number, patch: Partial<AutomationAction>): void {
    this.rule.update((r) => ({
      ...r,
      actions: replaceAt(r.actions, index, { ...r.actions[index], ...patch }),
    }));
  }

  protected setWho(index: number, event: Event): void {
    this.patchAction(index, { who: this.value(event), userId: undefined, teamId: undefined });
  }

  protected removeAction(index: number): void {
    this.rule.update((r) => ({ ...r, actions: removeAt(r.actions, index) }));
  }

  protected moveAction(index: number, delta: -1 | 1): void {
    this.rule.update((r) => ({ ...r, actions: moveItem(r.actions, index, delta) }));
  }

  // Smart values.

  protected rememberText(event: Event, index: number): void {
    this.lastText = { el: event.target as HTMLTextAreaElement | HTMLInputElement, index };
  }

  /** Writes `{{key}}` where the caret of the last text control was; at the end when none was used. */
  protected insertSmart(key: string, fallbackIndex: number): void {
    const token = `{{${key}}}`;
    const target = this.lastText?.index === fallbackIndex ? this.lastText : null;
    const index = fallbackIndex;
    const action = this.rule().actions[index];
    if (!action) return;
    const field: 'text' | 'value' = action.type === 'set_field' ? 'value' : 'text';
    const current = action[field] ?? '';
    const start = target?.el.selectionStart ?? current.length;
    const end = target?.el.selectionEnd ?? current.length;
    const next = current.slice(0, start) + token + current.slice(end);
    this.patchAction(index, { [field]: next });
    if (target) {
      const caret = start + token.length;
      queueMicrotask(() => {
        target.el.focus();
        target.el.setSelectionRange(caret, caret);
      });
    }
  }

  protected acceptsSmart(action: AutomationAction): boolean {
    return (
      action.type === 'add_comment' ||
      action.type === 'create_subtask' ||
      action.type === 'send_notification' ||
      action.type === 'set_field'
    );
  }
}
