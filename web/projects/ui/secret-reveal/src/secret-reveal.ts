import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { Button } from 'primeng/button';

/**
 * One-time reveal of a secret (access token, signing secret): the value in a read-only monospace field with a
 * copy button and a warning that it will not be shown again. It can only be dismissed after the person confirms
 * having copied it. The component keeps no copy of the value: it renders the input while it is bound and the
 * parent clears it on `dismissed`. The value is never logged or written to storage.
 */
@Component({
  selector: 'app-secret-reveal',
  imports: [Button],
  templateUrl: './secret-reveal.html',
  styleUrl: './secret-reveal.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SecretReveal {
  /** Plain value to show. Null or empty hides the panel. */
  readonly secret = input<string | null>(null);
  /** Name of the secret, as in "Token". */
  readonly label = input('Secreto');
  readonly warning = input('No volverás a ver este valor. Guárdalo ahora en un lugar seguro.');
  readonly confirmText = input('Ya lo copié');
  /** Emits once the person confirmed the copy and closed the panel. */
  readonly dismissed = output<void>();

  protected readonly acknowledged = signal(false);
  protected readonly copied = signal(false);
  protected readonly visible = computed(() => !!this.secret());
  private readonly field = viewChild<ElementRef<HTMLInputElement>>('field');

  protected async copy(): Promise<void> {
    const value = this.secret();
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      this.copied.set(true);
    } catch {
      // Without clipboard permission the person can still copy by hand from the selected field.
      this.field()?.nativeElement.select();
      this.copied.set(false);
    }
  }

  protected selectAll(event: Event): void {
    (event.target as HTMLInputElement).select();
  }

  protected toggle(event: Event): void {
    this.acknowledged.set((event.target as HTMLInputElement).checked);
  }

  protected dismiss(): void {
    if (!this.acknowledged()) return;
    this.acknowledged.set(false);
    this.copied.set(false);
    this.dismissed.emit();
  }
}
