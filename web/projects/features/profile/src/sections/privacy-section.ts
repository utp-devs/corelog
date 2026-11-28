import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  COMMUNICATIONS_SLUG,
  ConsentStore,
  LegalStore,
  type AcceptedConsent,
  type LegalDocumentSummary,
} from '@corelog/core';
import { LegalDocumentDialog } from '@corelog/ui/legal-document-dialog';
import { Section } from '@corelog/ui/section';
import { ToneTag } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { ToggleSwitch } from 'primeng/toggleswitch';

interface DocumentRow {
  readonly document: LegalDocumentSummary;
  readonly accepted: AcceptedConsent | null;
}

/**
 * Privacy section: the legal documents with the version and date the account accepted, a link to read
 * each one, and the optional communications authorization, which can be revoked and granted again.
 * Required documents cannot be revoked; the section says so.
 */
@Component({
  selector: 'app-profile-privacy-section',
  imports: [Button, DatePipe, FormsModule, LegalDocumentDialog, Section, ToggleSwitch, ToneTag],
  templateUrl: './privacy-section.html',
  styleUrl: './privacy-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfilePrivacySection {
  private readonly legal = inject(LegalStore);
  private readonly consents = inject(ConsentStore);

  protected readonly loading = this.consents.loading;
  protected readonly failure = signal<string | null>(null);
  protected readonly reading = signal<string | null>(null);

  protected readonly rows = computed<readonly DocumentRow[]>(() => {
    const accepted = new Map((this.consents.status()?.accepted ?? []).map((a) => [a.slug, a]));
    return this.legal
      .documents()
      .filter((d) => d.slug !== COMMUNICATIONS_SLUG)
      .map((document) => ({ document, accepted: accepted.get(document.slug) ?? null }));
  });
  protected readonly communications = this.legal.communications;
  protected readonly communicationsOn = computed(() =>
    (this.consents.status()?.accepted ?? []).some((a) => a.slug === COMMUNICATIONS_SLUG),
  );
  protected readonly ready = computed(() => this.consents.status() !== null);
  protected readonly readingVersion = computed(() => {
    const slug = this.reading();
    return slug ? this.legal.find(slug)?.version : undefined;
  });

  constructor() {
    void this.legal.load();
    void this.consents.load();
  }

  /** True when the accepted version is older than the current one. */
  protected outdated(row: DocumentRow): boolean {
    return row.accepted !== null && row.accepted.version < row.document.version;
  }

  protected async toggleCommunications(on: boolean): Promise<void> {
    const document = this.communications();
    if (!document) return;
    this.failure.set(null);
    try {
      if (on) {
        await this.consents.accept([{ slug: document.slug, version: document.version }], 'profile');
      } else {
        await this.consents.revoke(document.slug);
      }
    } catch {
      this.failure.set('No se pudo guardar tu elección. Inténtalo de nuevo.');
      await this.consents.load();
    }
  }
}
