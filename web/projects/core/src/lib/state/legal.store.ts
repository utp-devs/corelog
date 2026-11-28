import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import {
  COMMUNICATIONS_SLUG,
  type LegalDocument,
  type LegalDocumentSummary,
} from '../models/legal.model';
import { LegalApiService } from '../services/api/legal-api.service';

/** Current legal documents (public) and the bodies already read, so a link opens at once the second time. */
@Injectable({ providedIn: 'root' })
export class LegalStore {
  private readonly api = inject(LegalApiService);
  private pending: Promise<void> | null = null;
  private readonly bodies = new Map<string, LegalDocument>();

  readonly documents = signal<readonly LegalDocumentSummary[]>([]);
  readonly status = signal<'idle' | 'loading' | 'ready' | 'error'>('idle');

  /** Required documents plus the optional ones, by slug. */
  readonly bySlug = computed(() => new Map(this.documents().map((d) => [d.slug, d])));
  readonly communications = computed(() => this.bySlug().get(COMMUNICATIONS_SLUG));

  find(slug: string): LegalDocumentSummary | undefined {
    return this.bySlug().get(slug);
  }

  load(force = false): Promise<void> {
    if (force) this.bodies.clear();
    if (!force && this.status() === 'ready') return Promise.resolve();
    this.pending ??= this.fetch();
    return this.pending;
  }

  /** Reads a document with its body. Cached by slug and version; rejects with the ApiError. */
  async read(slug: string, version?: number): Promise<LegalDocument> {
    const key = `${slug}@${version ?? 'current'}`;
    const cached = this.bodies.get(key);
    if (cached) return cached;
    const document = await firstValueFrom(this.api.getDocument(slug, version));
    this.bodies.set(key, document);
    return document;
  }

  private async fetch(): Promise<void> {
    this.status.set('loading');
    try {
      this.documents.set(await firstValueFrom(this.api.listDocuments()));
      this.status.set('ready');
    } catch {
      this.status.set('error');
    } finally {
      this.pending = null;
    }
  }
}
