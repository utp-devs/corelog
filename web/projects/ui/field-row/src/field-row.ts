import {
  ChangeDetectionStrategy,
  Component,
  Directive,
  computed,
  inject,
  input,
} from '@angular/core';

let nextFieldRowId = 0;

/**
 * Label, control, helper and error for forms and detail panels. The label is associated with the
 * control marked with `appFieldControl`, and helper or error text is linked through aria-describedby.
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
  readonly required = input(false);
  /** Helper text under the control. */
  readonly helper = input<string>();
  /** Validation message: replaces the helper text and marks the control invalid. */
  readonly error = input<string>();

  /** Id for the control. Also usable on PrimeNG components that accept an input id. */
  readonly controlId = `corelog-field-${nextFieldRowId++}`;
  /** Id of the label text; PrimeNG controls reference it through their labelled-by input. */
  readonly labelId = `${this.controlId}-label`;
  protected readonly helperId = `${this.controlId}-helper`;
  protected readonly errorId = `${this.controlId}-error`;

  /** Ids of the text that describes the control; empty when no text is shown. */
  readonly describedBy = computed(() => {
    if (this.error()) {
      return this.errorId;
    }
    return this.helper() !== undefined ? this.helperId : null;
  });
  readonly invalid = computed(() => !!this.error());
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
