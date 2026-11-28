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
import { Textarea } from 'primeng/textarea';

export const FILTER_NAME_MAX = 120;
export const FILTER_DESCRIPTION_MAX = 500;

export interface SaveFilterRequest {
  readonly name: string;
  readonly description: string;
  /** Overwrite the loaded filter instead of creating a new one. */
  readonly replace: boolean;
}

/** Asks for the name of a filter. When one is loaded and editable, it can be updated instead of copied. */
@Component({
  selector: 'app-save-filter-dialog',
  imports: [Button, Dialog, InputText, Textarea],
  templateUrl: './save-filter-dialog.html',
  styleUrl: './save-filter-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SaveFilterDialog {
  readonly open = input(false);
  /** Name and description of the loaded filter, when the user can edit it. */
  readonly current = input<{ readonly name: string; readonly description: string } | null>(null);
  readonly busy = input(false);
  readonly error = input<string | null>(null);
  readonly save = output<SaveFilterRequest>();
  readonly closeRequested = output<void>();

  protected readonly name = signal('');
  protected readonly description = signal('');
  protected readonly replace = signal(false);
  protected readonly nameMax = FILTER_NAME_MAX;
  protected readonly descriptionMax = FILTER_DESCRIPTION_MAX;
  protected readonly valid = computed(
    () => this.name().trim().length > 0 && this.name().length <= FILTER_NAME_MAX,
  );

  constructor() {
    effect(() => {
      if (!this.open()) return;
      const current = untracked(() => this.current());
      this.replace.set(current !== null);
      this.name.set(current?.name ?? '');
      this.description.set(current?.description ?? '');
    });
  }

  protected setReplace(replace: boolean): void {
    this.replace.set(replace);
    if (!replace && this.current()) this.name.set(`${this.current()?.name} (copia)`);
    if (replace && this.current()) this.name.set(this.current()?.name ?? '');
  }

  protected submit(): void {
    if (!this.valid() || this.busy()) return;
    this.save.emit({
      name: this.name().trim(),
      description: this.description().trim(),
      replace: this.replace(),
    });
  }
}
