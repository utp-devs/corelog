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
import { FIELD_DEFAULT_ERROR } from '@corelog/ui/field-row';

let nextAuthFieldId = 0;

/**
 * Field of the access screens with the label inside the box (small caption on top, value below) and an
 * optional icon on the right. The label is a real `<label>` tied to the control marked with
 * `appAuthControl`; validation and helper texts are linked through aria-describedby.
 *
 * An `input` or `app-password-input` child is drawn inside the box; any other child (for example a
 * strength meter) is drawn below it. Error timing follows the same rule as the shared field row: the
 * message of the first validation error appears once the control was touched or edited, or after submit.
 */
@Component({
  selector: 'app-auth-field',
  templateUrl: './auth-field.html',
  styleUrl: './auth-field.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthField {
  readonly label = input.required<string>();
  readonly required = input(false);
  /** PrimeIcons class drawn on the right, for example `pi-envelope`. Omit it when the control has its own button. */
  readonly icon = input<string>();
  readonly helper = input<string>();
  /** Server-side or explicit message; replaces the helper and wins over the control error. */
  readonly error = input<string>();
  readonly control = input<AbstractControl | null>(null);
  readonly messages = input<Readonly<Record<string, string>>>({});
  /** True after the owning form was submitted: errors of untouched controls become visible. */
  readonly submitted = input(false);

  readonly controlId = `corelog-auth-field-${nextAuthFieldId++}`;
  protected readonly helperId = `${this.controlId}-helper`;
  protected readonly errorId = `${this.controlId}-error`;

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

  protected readonly message = computed(() => this.error() || this.controlError() || undefined);

  /** Id of the text that describes the control; null when no text is shown. */
  readonly describedBy = computed(() => {
    if (this.message()) {
      return this.errorId;
    }
    return this.helper() ? this.helperId : null;
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

/** Marks the native input of an `app-auth-field` and receives its id and aria attributes. */
@Directive({
  selector: '[appAuthControl]',
  host: {
    '[attr.id]': 'field.controlId',
    '[attr.aria-describedby]': 'field.describedBy()',
    '[attr.aria-invalid]': 'field.invalid() ? "true" : null',
    '[attr.aria-required]': 'field.required() ? "true" : null',
  },
})
export class AuthControl {
  readonly field = inject(AuthField);
}
