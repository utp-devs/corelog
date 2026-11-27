import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { Button } from 'primeng/button';

/**
 * Load error with a retry action. The host uses role alert so screen readers announce it.
 * The retry button is shown only when `canRetry` is true and the `retry` output is bound.
 */
@Component({
  selector: 'app-error-state',
  imports: [Button],
  templateUrl: './error-state.html',
  styleUrl: './error-state.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ErrorState {
  readonly title = input('No se pudo cargar la información');
  readonly message = input.required<string>();
  readonly canRetry = input(false);
  readonly retry = output<void>();
}
