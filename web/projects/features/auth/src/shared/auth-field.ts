import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

/**
 * Label, projected control and helper or error text for the auth forms. The control is written by the
 * page; this component only renders the label and the message, which the page links with aria-describedby
 * through `messageId`.
 */
@Component({
  selector: 'app-auth-field',
  templateUrl: './auth-field.html',
  styleUrl: '../shared/_auth-form.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthField {
  readonly label = input.required<string>();
  readonly inputId = input.required<string>();
  /** Error text (replaces the helper) or helper text. Omit or pass null for none. */
  readonly message = input<string | null>(null);
  readonly tone = input<'error' | 'helper'>('helper');

  /** Id of the message element; pages reference it from aria-describedby when a message is shown. */
  readonly messageId = computed(() => `${this.inputId()}-message`);
}
