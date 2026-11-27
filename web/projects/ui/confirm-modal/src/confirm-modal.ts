import {
  ChangeDetectionStrategy,
  Component,
  DOCUMENT,
  effect,
  inject,
  input,
  output,
  untracked,
} from '@angular/core';
import { PrimeTemplate } from 'primeng/api';
import { Button } from 'primeng/button';
import { Dialog } from 'primeng/dialog';

/**
 * Confirmation of an action, with its consequence written in the body.
 * Closing with the X, Escape or the backdrop counts as cancel. Focus returns to the
 * element that was focused before the dialog opened.
 */
@Component({
  selector: 'app-confirm-modal',
  imports: [Button, Dialog, PrimeTemplate],
  templateUrl: './confirm-modal.html',
  styleUrl: './confirm-modal.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConfirmModal {
  private readonly document = inject(DOCUMENT);
  private opener: HTMLElement | null = null;

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

  constructor() {
    effect(() => {
      const open = this.open();
      untracked(() => this.trackFocus(open));
    });
  }

  onConfirm(): void {
    if (this.loading()) return;
    this.confirmed.emit();
  }

  onCancel(): void {
    this.cancelled.emit();
  }

  /** Dialog reports a user close (X, Escape, backdrop). Programmatic closes come from `open`. */
  onVisibleChange(visible: boolean): void {
    if (!visible) this.cancelled.emit();
  }

  private trackFocus(open: boolean): void {
    if (open) {
      const active = this.document.activeElement;
      this.opener = active instanceof HTMLElement ? active : null;
    } else if (this.opener) {
      this.opener.focus();
      this.opener = null;
    }
  }
}
