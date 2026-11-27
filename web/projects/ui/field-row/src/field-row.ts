import {
  ChangeDetectionStrategy,
  Component,
  Directive,
  computed,
  effect,
  inject,
  input,
  signal,
} from '@angular/core';
import type { AbstractControl } from '@angular/forms';

let nextFieldRowId = 0;

/** Fallback text when a control is invalid and `messages` has no text for its first error. */
export const FIELD_DEFAULT_ERROR = 'Revisa este campo.';

/**
 * Label, control, helper and error for forms and detail panels. The label is associated with the
 * control marked with `appFieldControl`, and helper or error text is linked through aria-describedby.
 *
 * Error timing: pass `control` (and optionally `messages` and `submitted`) and the row shows the message
 * of the first validation error only after the control was touched or edited, or after the form was
 * submitted. An empty untouched field never shows an error. The plain `error` input still works and
 * always wins.
 */
@Component({
  selector: 'app-field-row',
  templateUrl: './field-row.html',
  styleUrl: './field-row.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FieldRow {
  readonly label = input.required<string>();
  /** Label above the control (forms, default) or beside it (detail panels). */
  readonly variant = input<'stacked' | 'inline'>('stacked');
  /** Marks the field as obligatory: asterisk for sighted users and "(obligatorio)" for screen readers. */
  readonly required = input(false);
  /** Shows a muted "(opcional)" next to the label. Use it in forms where most fields are required. */
  readonly optional = input(false);
  /** Helper text under the control. */
  readonly helper = input<string>();
  /** Validation message: replaces the helper text and marks the control invalid. */
  readonly error = input<string>();
  /** Reason why the control is disabled. Shown under the control with a lock icon instead of the helper. */
  readonly disabledReason = input<string>();
  /** Form control whose validity drives the error message (shown only after touch or submit). */
  readonly control = input<AbstractControl | null>(null);
  /** Message per validation error key, for example `{ required: 'Escribe un título.' }`. */
  readonly messages = input<Readonly<Record<string, string>>>({});
  /** True after the owning form was submitted: errors of untouched controls become visible. */
  readonly submitted = input(false);

  /** Id for the control. Also usable on PrimeNG components that accept an input id. */
  readonly controlId = `corelog-field-${nextFieldRowId++}`;
  /** Id of the label text; PrimeNG controls reference it through their labelled-by input. */
  readonly labelId = `${this.controlId}-label`;
  protected readonly helperId = `${this.controlId}-helper`;
  protected readonly errorId = `${this.controlId}-error`;

  /** Bumps on every control event so the computed error follows touched, dirty and status changes. */
  private readonly controlTick = signal(0);

  private readonly controlError = computed(() => {
    this.controlTick();
    const control = this.control();
    if (!control?.invalid) {
      return undefined;
    }
    if (!(control.touched || control.dirty || this.submitted())) {
      return undefined;
    }
    const key = Object.keys(control.errors ?? {})[0] ?? '';
    const messages = this.messages();
    return messages[key] ?? messages['default'] ?? FIELD_DEFAULT_ERROR;
  });

  /** The visible error: the explicit `error` input, else the control error once it may be shown. */
  protected readonly message = computed(() => this.error() || this.controlError() || undefined);
  /** Text under the control when there is no error: the disabled reason, else the helper. */
  protected readonly hint = computed(() => this.disabledReason() || this.helper());
  protected readonly hintIcon = computed(() =>
    this.disabledReason() ? 'pi pi-lock' : 'pi pi-info-circle',
  );

  /** Id of the text that describes the control; null when no text is shown. */
  readonly describedBy = computed(() => {
    if (this.message()) {
      return this.errorId;
    }
    const hint = this.hint();
    return hint !== undefined && hint !== '' ? this.helperId : null;
  });
  readonly invalid = computed(() => !!this.message());

  constructor() {
    effect((onCleanup) => {
      const control = this.control();
      if (!control) {
        return;
      }
      const subscription = control.events.subscribe(() => this.controlTick.update((n) => n + 1));
      onCleanup(() => subscription.unsubscribe());
    });
  }
}

/** Marks the native control inside an `app-field-row` and receives its ids and aria attributes. */
@Directive({
  selector: '[appFieldControl]',
  host: {
    '[attr.id]': 'row.controlId',
    '[attr.aria-describedby]': 'row.describedBy()',
    '[attr.aria-invalid]': 'row.invalid() ? "true" : null',
    '[attr.aria-required]': 'row.required() ? "true" : null',
  },
})
export class FieldControl {
  readonly row = inject(FieldRow);
}
