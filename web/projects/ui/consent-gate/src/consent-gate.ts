import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AuthStore, ConsentStore, asApiError } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { LegalConsentCheckbox } from '@corelog/ui/legal-consent-checkbox';
import { LegalDocumentDialog } from '@corelog/ui/legal-document-dialog';
import { Button } from 'primeng/button';

/**
 * Blocking modal shown when the signed-in account has required legal documents whose current version it
 * has not accepted. It cannot be dismissed (no Escape, no click outside, no close button): the person
 * reads and ticks each document, accepts and continues, or signs out. Mounted once in the app root.
 */
@Component({
  selector: 'app-consent-gate',
  imports: [Button, Dialog, FormsModule, LegalConsentCheckbox, LegalDocumentDialog],
  templateUrl: './consent-gate.html',
  styleUrl: './consent-gate.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConsentGate {
  private readonly consents = inject(ConsentStore);
  private readonly auth = inject(AuthStore);

  protected readonly pending = this.consents.pending;
  protected readonly open = this.consents.hasPending;
  protected readonly loading = this.consents.loading;
  protected readonly ticked = signal<ReadonlySet<string>>(new Set());
  protected readonly reading = signal<string | null>(null);
  protected readonly failure = signal<string | null>(null);

  protected readonly readingVersion = computed(
    () => this.pending().find((p) => p.slug === this.reading())?.version,
  );
  protected readonly allTicked = computed(
    () => this.pending().length > 0 && this.pending().every((p) => this.ticked().has(p.slug)),
  );

  constructor() {
    // A new set of pending documents (another version published) starts with nothing ticked.
    effect(() => {
      this.pending();
      untracked(() => {
        this.ticked.set(new Set());
        this.failure.set(null);
      });
    });
  }

  protected isTicked(slug: string): boolean {
    return this.ticked().has(slug);
  }

  protected toggle(slug: string, value: boolean): void {
    const next = new Set(this.ticked());
    if (value) next.add(slug);
    else next.delete(slug);
    this.ticked.set(next);
  }

  protected acceptFromDialog(slug: string): void {
    this.toggle(slug, true);
    this.reading.set(null);
  }

  protected async accept(): Promise<void> {
    if (!this.allTicked() || this.loading()) return;
    this.failure.set(null);
    try {
      await this.consents.accept(
        this.pending().map((p) => ({ slug: p.slug, version: p.version })),
        'login_reprompt',
      );
    } catch (error) {
      this.failure.set(
        asApiError(error).message || 'No se pudo registrar tu aceptación. Inténtalo de nuevo.',
      );
    }
  }

  protected logout(): void {
    this.auth.logout();
  }
}
