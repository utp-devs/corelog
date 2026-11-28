import {
  ChangeDetectionStrategy,
  Component,
  computed,
  forwardRef,
  inject,
  input,
  signal,
} from '@angular/core';
import {
  AbstractControl,
  ControlValueAccessor,
  FormsModule,
  NG_VALIDATORS,
  NG_VALUE_ACCESSOR,
  ValidationErrors,
  Validator,
} from '@angular/forms';
import {
  COMMUNICATIONS_SLUG,
  LegalStore,
  REQUIRED_CONSENT_SLUGS,
  type ConsentRef,
  type LegalDocumentSummary,
} from '@corelog/core';
import { LegalDocumentDialog } from '@corelog/ui/legal-document-dialog';
import { Button } from 'primeng/button';

import { LegalConsentCheckbox } from './legal-consent-checkbox';

export interface ConsentCopy {
  readonly slug: string;
  readonly before: string;
  readonly linkLabel: string;
  readonly after: string;
  readonly error: string;
}

/** Wording of each checkbox, in display order. A document that the API does not publish is skipped. */
export const CONSENT_COPY: readonly ConsentCopy[] = [
  {
    slug: 'privacy_policy',
    before: 'He leído y acepto la',
    linkLabel: 'Política de privacidad',
    after: '',
    error: 'Debes aceptar la política de privacidad para crear la cuenta.',
  },
  {
    slug: 'data_processing',
    before: 'Autorizo el',
    linkLabel: 'tratamiento de mis datos personales',
    after: 'según la política.',
    error: 'Debes autorizar el tratamiento de tus datos personales para crear la cuenta.',
  },
  {
    slug: 'terms_of_use',
    before: 'Acepto los',
    linkLabel: 'Términos de uso',
    after: '',
    error: 'Debes aceptar los términos de uso para crear la cuenta.',
  },
  {
    slug: COMMUNICATIONS_SLUG,
    before: 'Quiero recibir',
    linkLabel: 'comunicaciones y novedades del producto',
    after: '',
    error: '',
  },
];

/**
 * The consent checkboxes of the sign-up: the three required documents and the optional communications
 * authorization. Each link opens the document in a modal, whose "Aceptar y cerrar" also ticks the box.
 * The value is the list of accepted `{ slug, version }` pairs, with the version that is shown; the control
 * is invalid until every required document is ticked.
 */
@Component({
  selector: 'app-legal-consent-group',
  imports: [Button, FormsModule, LegalConsentCheckbox, LegalDocumentDialog],
  templateUrl: './legal-consent-group.html',
  styleUrl: './legal-consent-group.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => LegalConsentGroup), multi: true },
    { provide: NG_VALIDATORS, useExisting: forwardRef(() => LegalConsentGroup), multi: true },
  ],
})
export class LegalConsentGroup implements ControlValueAccessor, Validator {
  private readonly store = inject(LegalStore);

  /** Shows the per-box errors (after a submit attempt). */
  readonly showErrors = input(false);
  /** Slugs the server reported as missing. */
  readonly serverMissing = input<readonly string[]>([]);

  protected readonly status = this.store.status;
  protected readonly checked = signal<ReadonlySet<string>>(new Set());
  protected readonly reading = signal<string | null>(null);
  protected readonly disabled = signal(false);

  /** Boxes to draw: the copy whose document the API publishes. */
  protected readonly items = computed(() =>
    CONSENT_COPY.flatMap((copy) => {
      const document = this.store.find(copy.slug);
      return document ? [{ copy, document }] : [];
    }),
  );
  protected readonly readingVersion = computed(() => {
    const slug = this.reading();
    return slug ? this.store.find(slug)?.version : undefined;
  });

  private onChange: (value: ConsentRef[]) => void = () => undefined;
  private onTouched: () => void = () => undefined;
  private onValidatorChange: () => void = () => undefined;

  constructor() {
    void this.store.load().then(() => this.onValidatorChange());
  }

  writeValue(value: readonly ConsentRef[] | null): void {
    this.checked.set(new Set((value ?? []).map((ref) => ref.slug)));
  }

  registerOnChange(fn: (value: ConsentRef[]) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  registerOnValidatorChange(fn: () => void): void {
    this.onValidatorChange = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled.set(isDisabled);
  }

  /** Invalid while a required document is not ticked. Key `consentsMissing` lists the slugs. */
  validate(_control: AbstractControl): ValidationErrors | null {
    const missing = this.missing();
    return missing.length > 0 ? { consentsMissing: missing } : null;
  }

  protected retry(): void {
    void this.store.load(true).then(() => this.onValidatorChange());
  }

  protected isChecked(slug: string): boolean {
    return this.checked().has(slug);
  }

  protected errorFor(copy: ConsentCopy, document: LegalDocumentSummary): string | null {
    if (!document.required || this.isChecked(copy.slug)) return null;
    const flagged =
      this.serverMissing().includes(copy.slug) ||
      (this.showErrors() && this.missing().includes(copy.slug));
    return flagged ? copy.error : null;
  }

  protected toggle(slug: string, value: boolean): void {
    const next = new Set(this.checked());
    if (value) next.add(slug);
    else next.delete(slug);
    this.checked.set(next);
    this.emit();
  }

  /** Accept from the dialog: ticks the box and closes it. */
  protected acceptFromDialog(slug: string): void {
    this.toggle(slug, true);
    this.reading.set(null);
  }

  private missing(): string[] {
    const required = this.store
      .documents()
      .filter((d) => d.required)
      .map((d) => d.slug);
    const slugs = required.length > 0 ? required : [...REQUIRED_CONSENT_SLUGS];
    return slugs.filter((slug) => !this.checked().has(slug));
  }

  private emit(): void {
    const refs = this.items()
      .filter(({ copy }) => this.checked().has(copy.slug))
      .map(({ document }) => ({ slug: document.slug, version: document.version }));
    this.onChange(refs);
    this.onTouched();
  }
}
