import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  ASSIGN_TARGET_OPTIONS,
  DEFAULT_DONE_RESOLUTION,
  PERMISSION_OPTIONS,
  PROJECT_ROLE_OPTIONS,
  RuleKind,
  RuleParamSpec,
  STANDARD_FIELD_OPTIONS,
  WorkflowRule,
  newRule,
  ruleFieldError,
  ruleSpec,
  specsOfKind,
} from '@corelog/core';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Textarea } from 'primeng/textarea';

let nextRuleListId = 0;

/**
 * Editor of the conditions, validators or post-functions of a transition. Every rule gets a form generated
 * from the typed registry of rules: roles as checkboxes, permissions and targets as selects, fields and
 * values as text. The owner stores the list the component emits.
 */
@Component({
  selector: 'app-workflow-rule-list',
  imports: [AppTooltip, Button, FormsModule, InputText, SelectField, Textarea],
  templateUrl: './rule-list.html',
  styleUrl: './rule-list.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowRuleList {
  readonly kind = input.required<RuleKind>();
  readonly title = input.required<string>();
  readonly hint = input<string>();
  readonly rules = input.required<readonly WorkflowRule[]>();
  /** Resolutions of the organization, for the "set resolution" rule. */
  readonly resolutions = input<readonly SelectFieldOption[]>([]);
  readonly canEdit = input(false);

  readonly rulesChange = output<readonly WorkflowRule[]>();

  protected readonly listId = `workflow-rules-${nextRuleListId++}`;
  protected readonly roleOptions = PROJECT_ROLE_OPTIONS;
  protected readonly permissionOptions: readonly SelectFieldOption[] = PERMISSION_OPTIONS;
  protected readonly assignOptions: readonly SelectFieldOption[] = ASSIGN_TARGET_OPTIONS;
  protected readonly fieldOptions = STANDARD_FIELD_OPTIONS;
  protected readonly adding = signal<string | null>(null);

  protected readonly typeOptions = computed<SelectFieldOption[]>(() =>
    specsOfKind(this.kind()).map((spec) => ({ value: spec.type, label: spec.label })),
  );
  protected readonly resolutionOptions = computed<SelectFieldOption[]>(() => [
    { value: DEFAULT_DONE_RESOLUTION, label: 'Hecho (resolución predeterminada)' },
    ...this.resolutions(),
  ]);

  protected specOf(rule: WorkflowRule): {
    label: string;
    description: string;
    params: readonly RuleParamSpec[];
  } {
    const spec = ruleSpec(rule.type);
    return {
      label: spec?.label ?? rule.type,
      description: spec?.description ?? '',
      params: spec?.params ?? [],
    };
  }

  protected errorOf(rule: WorkflowRule): string | null {
    return ruleFieldError(rule);
  }

  protected valueOf(rule: WorkflowRule, key: string): string {
    const value = rule.params[key];
    return typeof value === 'string' ? value : '';
  }

  protected rolesOf(rule: WorkflowRule): readonly string[] {
    const value = rule.params['roles'];
    return Array.isArray(value) ? (value as string[]) : [];
  }

  protected add(type: string | null): void {
    if (!type) return;
    this.rulesChange.emit([...this.rules(), newRule(type)]);
    // Reset the select so the same type can be added again.
    this.adding.set(null);
  }

  protected remove(index: number): void {
    this.rulesChange.emit(this.rules().filter((_, position) => position !== index));
  }

  protected setType(index: number, type: string | null): void {
    if (!type) return;
    this.replace(index, newRule(type));
  }

  protected setParam(index: number, key: string, value: unknown): void {
    const rule = this.rules()[index];
    this.replace(index, { ...rule, params: { ...rule.params, [key]: value } });
  }

  protected toggleRole(index: number, role: string, checked: boolean): void {
    const current = this.rolesOf(this.rules()[index]);
    const next = checked ? [...current, role] : current.filter((item) => item !== role);
    this.setParam(index, 'roles', next);
  }

  private replace(index: number, rule: WorkflowRule): void {
    this.rulesChange.emit(this.rules().map((item, position) => (position === index ? rule : item)));
  }
}
