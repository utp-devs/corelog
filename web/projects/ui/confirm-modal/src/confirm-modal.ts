import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';

/**
 * Confirmation of an action, with its consequence written in the body.
 * Closing with the X, Escape or the backdrop counts as cancel. Focus starts on the cancel button and
 * returns to the element that was focused before the dialog opened.
 */
@Component({
  selector: 'app-confirm-modal',
  imports: [Button, Dialog],
  templateUrl: './confirm-modal.html',
  styleUrl: './confirm-modal.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConfirmModal {
  readonly open = input(false);
  readonly title = input.required<string>();
  /** Consequence of the action, visible before confirming. */
  readonly description = input<string>();
  readonly confirmText = input('Confirmar');
  readonly cancelText = input('Cancelar');
  /** Marks the action as destructive: the confirm button uses the danger style. */
  readonly danger = input(false);
  readonly loading = input(false);
  readonly confirmed = output<void>();
  readonly cancelled = output<void>();

  onConfirm(): void {
    if (this.loading()) return;
    this.confirmed.emit();
  }

  onCancel(): void {
    this.cancelled.emit();
  }
}
