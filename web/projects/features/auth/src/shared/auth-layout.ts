import { ChangeDetectionStrategy, Component, input } from '@angular/core';

let nextLayoutId = 0;

/** Id of the main landmark; the skip link points here. */
export const AUTH_MAIN_ID = 'auth-main';

/**
 * Shared template for the public auth screens. A single centered card on the token gradient, with the
 * brand banner at its top on every viewport. The footer slot is projected with the `authFooter` attribute.
 */
@Component({
  selector: 'app-auth-layout',
  templateUrl: './auth-layout.html',
  styleUrl: './auth-layout.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthLayout {
  readonly title = input.required<string>();
  readonly subtitle = input<string | null>(null);

  protected readonly mainId = AUTH_MAIN_ID;
  protected readonly titleId = `auth-title-${nextLayoutId++}`;
}
