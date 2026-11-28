import { DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { LegalStore, asApiError, type ConsentRef, type LegalDocument } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Button } from 'primeng/button';

type LoadState =
  | { readonly kind: 'idle' }
  | { readonly kind: 'loading' }
  | { readonly kind: 'error'; readonly message: string }
  | { readonly kind: 'ready'; readonly document: LegalDocument };

/**
 * Modal that shows a legal document: title, version, effective date and the Markdown body rendered
 * through the sanitized rich text view. Open it by giving a `slug`; it fetches the current version (or
 * `version`). With `acceptable` it also offers "Aceptar y cerrar", which emits `accepted` with the exact
 * version that was read. The shell owns focus and returns it to the element that opened the dialog.
 */
@Component({
  selector: 'app-legal-document-dialog',
  imports: [Button, DatePipe, Dialog, RichTextView],
  templateUrl: './legal-document-dialog.html',
  styleUrl: './legal-document-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LegalDocumentDialog {
  private readonly store = inject(LegalStore);

  /** Slug of the document to show; null keeps the dialog closed. */
  readonly slug = input<string | null>(null);
  readonly version = input<number | undefined>(undefined);
  /** Offers the accept action. Without it the dialog is read only. */
  readonly acceptable = input(false);

  readonly closed = output<void>();
  readonly accepted = output<ConsentRef>();

  protected readonly state = signal<LoadState>({ kind: 'idle' });
  protected readonly open = computed(() => this.slug() !== null);
  protected readonly heading = computed(() => {
    const current = this.state();
    return current.kind === 'ready' ? current.document.title : 'Documento legal';
  });
  protected readonly document = computed(() => {
    const current = this.state();
    return current.kind === 'ready' ? current.document : null;
  });

  private requestId = 0;

  constructor() {
    effect(() => {
      const slug = this.slug();
      const version = this.version();
      untracked(() => (slug ? void this.load(slug, version) : this.state.set({ kind: 'idle' })));
    });
  }

  protected retry(): void {
    const slug = this.slug();
    if (slug) void this.load(slug, this.version());
  }

  protected accept(): void {
    const document = this.document();
    if (document) this.accepted.emit({ slug: document.slug, version: document.version });
  }

  private async load(slug: string, version: number | undefined): Promise<void> {
    const id = ++this.requestId;
    this.state.set({ kind: 'loading' });
    try {
      const document = await this.store.read(slug, version);
      if (id === this.requestId) this.state.set({ kind: 'ready', document });
    } catch (error) {
      if (id === this.requestId) {
        this.state.set({
          kind: 'error',
          message: asApiError(error).message || 'No se pudo cargar el documento.',
        });
      }
    }
  }
}
