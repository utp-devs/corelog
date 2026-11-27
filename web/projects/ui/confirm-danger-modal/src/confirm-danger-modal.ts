import {
  ChangeDetectionStrategy,
  Component,
  DOCUMENT,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { PrimeTemplate } from 'primeng/api';
import { Button } from 'primeng/button';
import { Dialog } from 'primeng/dialog';

import { isConfirmationMatch } from './confirmation';

let nextInputId = 0;

/**
 * Confirmation of a destructive action: the confirm button stays disabled until the user
 * types the exact name shown in the prompt. The input takes focus when the dialog opens.
 */
@Component({
  selector: 'app-confirm-danger-modal',
  imports: [Button, Dialog, PrimeTemplate],
  templateUrl: './confirm-danger-modal.html',
  styleUrl: './confirm-danger-modal.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConfirmDangerModal {
  private readonly document = inject(DOCUMENT);
  private opener: HTMLElement | null = null;

  readonly open = input(false);
  readonly title = input.required<string>();
  /** Consequence of the action, visible before confirming. */
  readonly description = input<string>();
  /** Exact text the user must type (project or organization name). */
  readonly confirmation = input.required<string>();
  readonly confirmText = input('Eliminar');
  readonly loading = input(false);
  readonly confirmed = output<void>();
  readonly cancelled = output<void>();

  protected readonly typed = signal('');
  protected readonly matches = computed(() =>
    isConfirmationMatch(this.typed(), this.confirmation()),
  );
  protected readonly fieldId = `cl-confirm-danger-${nextInputId++}`;

  constructor() {
    effect(() => {
      const open = this.open();
      untracked(() => this.trackFocus(open));
    });
  }

  protected onTyped(value: string): void {
    this.typed.set(value);
  }

  protected onConfirm(): void {
    if (!this.matches() || this.loading()) return;
    this.confirmed.emit();
  }

  protected onCancel(): void {
    this.cancelled.emit();
  }

  /** Dialog reports a user close (X, Escape, backdrop). */
  protected onVisibleChange(visible: boolean): void {
    if (!visible) this.cancelled.emit();
  }

  /** Runs after the dialog has left: the typed text is discarded. */
  protected onDialogHidden(): void {
    this.typed.set('');
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
