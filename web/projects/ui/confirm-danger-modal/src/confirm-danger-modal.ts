import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import { isConfirmationMatch } from './confirmation';

let nextInputId = 0;

/**
 * Confirmation of a destructive action: the confirm button stays disabled until the user
 * types the exact name shown in the prompt. The input takes focus when the dialog opens.
 */
@Component({
  selector: 'app-confirm-danger-modal',
  imports: [Button, Dialog, InputText],
  templateUrl: './confirm-danger-modal.html',
  styleUrl: './confirm-danger-modal.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConfirmDangerModal {
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
    // The typed text is discarded whenever the dialog closes.
    effect(() => {
      if (!this.open()) untracked(() => this.typed.set(''));
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
}
