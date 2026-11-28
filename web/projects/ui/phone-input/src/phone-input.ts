import {
  CdkConnectedOverlay,
  CdkOverlayOrigin,
  type ConnectedPosition,
} from '@angular/cdk/overlay';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  forwardRef,
  inject,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import {
  AbstractControl,
  ControlValueAccessor,
  NG_VALIDATORS,
  NG_VALUE_ACCESSOR,
  ValidationErrors,
  Validator,
} from '@angular/forms';
import {
  CountryStore,
  flagOf,
  formatNational,
  isPlausibleNational,
  maxNationalDigits,
  parseInternational,
  phoneDigits,
  phonePlaceholder,
  toE164,
  type Country,
  type PhoneValue,
} from '@corelog/core';

/** Gap in px between the field and its country panel. */
const PANEL_GAP = 4;

/** Panel below the field, or above it when there is no room below. */
const PANEL_POSITIONS: ConnectedPosition[] = [
  { originX: 'start', originY: 'bottom', overlayX: 'start', overlayY: 'top', offsetY: PANEL_GAP },
  { originX: 'start', originY: 'top', overlayX: 'start', overlayY: 'bottom', offsetY: -PANEL_GAP },
];

/**
 * Tokens the panel needs. The panel is rendered in the overlay container, outside the page scope that may re-map
 * them (the access screens do), so the values the field sees are copied onto the panel when it opens.
 */
const PANEL_TOKENS: readonly string[] = [
  '--cl-bg-elevated',
  '--cl-bg-surface',
  '--cl-bg-sunken',
  '--cl-bg-hover',
  '--cl-bg-selected',
  '--cl-border',
  '--cl-border-strong',
  '--cl-text',
  '--cl-text-subtle',
  '--cl-text-placeholder',
  '--cl-text-disabled',
  '--cl-brand',
  '--cl-danger',
  '--cl-shadow-focus',
  '--cl-control-h-md',
];

let nextPhoneInputId = 0;

/** Text without accents and case, to match what the user types against country names. */
function plain(text: string): string {
  return text.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
}

/**
 * Phone field: a searchable country selector (flag, name and prefix) beside the national number.
 * The number is spaced while typing, a pasted international number (+51 912 345 678) switches the
 * country by its prefix (countries that share +1 are told apart by area code), and the value is the
 * pair `{ country, number }` with national digits. The E.164 text is emitted through `e164Change`.
 * Works with reactive forms and validates the length for the country; the server stays the authority.
 */
@Component({
  selector: 'app-phone-input',
  templateUrl: './phone-input.html',
  styleUrl: './phone-input.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CdkConnectedOverlay, CdkOverlayOrigin],
  host: { '(document:mousedown)': 'onDocumentPress($event)' },
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => PhoneInput), multi: true },
    { provide: NG_VALIDATORS, useExisting: forwardRef(() => PhoneInput), multi: true },
  ],
})
export class PhoneInput implements ControlValueAccessor, Validator {
  private readonly store = inject(CountryStore);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly field = viewChild<ElementRef<HTMLInputElement>>('field');
  private readonly trigger = viewChild<ElementRef<HTMLButtonElement>>('trigger');
  private readonly search = viewChild<ElementRef<HTMLInputElement>>('search');
  private readonly panel = viewChild<ElementRef<HTMLElement>>('panel');
  private readonly injector = inject(Injector);

  /** Id of the national number input; the caption of the surrounding field points to it. */
  readonly inputId = input.required<string>();
  readonly size = input<'md' | 'lg'>('md');
  readonly invalid = input(false);
  readonly required = input(false);
  readonly describedBy = input<string | null>(null);
  /** Accessible name of the number input when no visible label points to it. */
  readonly ariaLabel = input<string | null>(null);

  /** E.164 text of the current value, empty while the number is incomplete. */
  readonly e164Change = output<string>();

  protected readonly flagOf = flagOf;
  protected readonly listId = `cl-phone-list-${nextPhoneInputId++}`;
  protected readonly countries = this.store.countries;
  protected readonly countryCode = signal<string | null>(null);
  protected readonly digits = signal('');
  /** Text shown while a number that starts with + is still being typed. */
  private readonly draft = signal<string | null>(null);
  protected readonly disabled = signal(false);
  protected readonly open = signal(false);
  protected readonly query = signal('');
  protected readonly active = signal(0);
  protected readonly positions = PANEL_POSITIONS;
  /** Width of the field when the panel opened: the panel is never narrower. */
  protected readonly minWidth = signal(0);

  protected readonly country = computed(() => this.store.find(this.countryCode()));
  protected readonly flag = computed(() => flagOf(this.countryCode() ?? ''));
  protected readonly dialCode = computed(() => this.country()?.dialCode ?? '+');
  protected readonly placeholder = computed(() => phonePlaceholder(this.country()));
  protected readonly shown = computed(
    () => this.draft() ?? formatNational(this.digits(), this.country()),
  );
  protected readonly maxLength = computed(() => maxNationalDigits(this.country()) * 2 + 2);
  protected readonly triggerLabel = computed(() => {
    const country = this.country();
    return country ? `País: ${country.name} ${country.dialCode}` : 'Elegir país';
  });

  protected readonly filtered = computed<readonly Country[]>(() => {
    const query = plain(this.query().trim());
    const all = this.countries();
    if (!query) return all;
    const numeric = query.replace(/[^\d+]/g, '');
    return all.filter(
      (c) =>
        plain(c.name).includes(query) ||
        c.code.toLowerCase() === query ||
        (numeric.length > 0 && c.dialCode.replace('+', '').startsWith(numeric.replace('+', ''))),
    );
  });
  protected readonly activeId = computed(() => {
    const list = this.filtered();
    const country = list[Math.min(this.active(), list.length - 1)];
    return country ? `${this.listId}-${country.code}` : null;
  });

  private onChange: (value: PhoneValue | null) => void = () => undefined;
  private onTouched: () => void = () => undefined;
  private onValidatorChange: () => void = () => undefined;
  /** Value written by the form before the catalog arrived. */
  private written: PhoneValue | null = null;

  constructor() {
    void this.store.load();
    // When the page or a container scrolls the panel would drift from the field: close it.
    effect((onCleanup) => {
      if (!this.open()) return;
      const close = (event: Event): void => {
        if (!this.panel()?.nativeElement.contains(event.target as Node)) this.close(false);
      };
      document.addEventListener('scroll', close, true);
      onCleanup(() => document.removeEventListener('scroll', close, true));
    });
    effect(() => {
      const ready = this.countries().length > 0;
      untracked(() => {
        if (!ready) return;
        if (this.written) {
          this.countryCode.set(this.written.country);
          this.written = null;
        } else if (!this.countryCode()) {
          const languages = typeof navigator === 'undefined' ? [] : navigator.languages;
          this.countryCode.set(this.store.defaultCountry(languages)?.code ?? null);
        }
        this.onValidatorChange();
      });
    });
  }

  writeValue(value: PhoneValue | null): void {
    this.draft.set(null);
    if (value) {
      this.countryCode.set(value.country.toUpperCase());
      this.digits.set(phoneDigits(value.number));
      if (!this.store.find(value.country)) this.written = value;
    } else {
      this.digits.set('');
    }
  }

  registerOnChange(fn: (value: PhoneValue | null) => void): void {
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

  /** Invalid when digits were typed and the length does not fit the country. Empty is left to `required`. */
  validate(_control: AbstractControl): ValidationErrors | null {
    if (this.draft() !== null) return { phone: true };
    const digits = this.digits();
    if (!digits) return null;
    return isPlausibleNational(digits, this.country()) ? null : { phone: true };
  }

  protected onInput(event: Event): void {
    const input = event.target as HTMLInputElement;
    const raw = input.value;
    const caretDigits = phoneDigits(raw.slice(0, input.selectionStart ?? raw.length)).length;
    const international = parseInternational(raw, this.countries());
    if (international) {
      this.draft.set(null);
      this.countryCode.set(international.country.code);
      this.digits.set(this.capped(international.national, international.country));
    } else if (/^\s*(\+|00)/.test(raw)) {
      // A prefix still being typed: keep what the person sees until it names a country.
      this.draft.set(raw);
      this.digits.set('');
      this.emit();
      return;
    } else {
      this.draft.set(null);
      this.digits.set(this.capped(phoneDigits(raw), this.country()));
    }
    const text = this.shown();
    input.value = text;
    this.placeCaret(input, caretDigits, text);
    this.emit();
  }

  protected onBlur(): void {
    this.onTouched();
  }

  protected toggle(): void {
    if (this.disabled()) return;
    if (this.open()) this.close(true);
    else this.openPanel();
  }

  private openPanel(): void {
    this.query.set('');
    const index = this.countries().findIndex((c) => c.code === this.countryCode());
    this.active.set(Math.max(index, 0));
    this.minWidth.set(this.host.nativeElement.getBoundingClientRect().width);
    this.open.set(true);
  }

  /** The panel is in the DOM: copy the tokens, focus the search and bring the current country into view. */
  protected onAttach(): void {
    afterNextRender(
      () => {
        const panel = this.panel()?.nativeElement;
        if (panel) {
          const style = getComputedStyle(this.host.nativeElement);
          for (const token of PANEL_TOKENS) {
            const value = style.getPropertyValue(token);
            if (value) panel.style.setProperty(token, value);
          }
        }
        this.search()?.nativeElement.focus();
        this.scrollToActive();
      },
      { injector: this.injector },
    );
  }

  protected close(focusTrigger: boolean): void {
    this.open.set(false);
    if (focusTrigger) this.trigger()?.nativeElement.focus();
  }

  protected onSearch(event: Event): void {
    this.query.set((event.target as HTMLInputElement).value);
    this.active.set(0);
  }

  protected onSearchKey(event: KeyboardEvent): void {
    const count = this.filtered().length;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        this.active.set(Math.min(this.active() + 1, count - 1));
        this.scrollToActive();
        break;
      case 'ArrowUp':
        event.preventDefault();
        this.active.set(Math.max(this.active() - 1, 0));
        this.scrollToActive();
        break;
      case 'Home':
        event.preventDefault();
        this.active.set(0);
        this.scrollToActive();
        break;
      case 'End':
        event.preventDefault();
        this.active.set(Math.max(count - 1, 0));
        this.scrollToActive();
        break;
      case 'Enter': {
        event.preventDefault();
        const country = this.filtered()[this.active()];
        if (country) this.choose(country);
        break;
      }
      case 'Escape':
        event.preventDefault();
        event.stopPropagation();
        this.close(true);
        break;
      case 'Tab':
        this.close(false);
        break;
    }
  }

  protected choose(country: Country): void {
    this.draft.set(null);
    this.countryCode.set(country.code);
    this.digits.set(this.capped(this.digits(), country));
    this.close(false);
    this.field()?.nativeElement.focus();
    this.emit();
  }

  protected onDocumentPress(event: MouseEvent): void {
    const target = event.target as Node;
    if (!this.open()) return;
    if (this.host.nativeElement.contains(target) || this.panel()?.nativeElement.contains(target))
      return;
    this.close(false);
  }

  private capped(digits: string, country: Country | undefined): string {
    return digits.slice(0, maxNationalDigits(country));
  }

  private emit(): void {
    const country = this.country();
    const digits = this.digits();
    this.onChange(country && digits ? { country: country.code, number: digits } : null);
    this.e164Change.emit(toE164(country, digits));
  }

  private placeCaret(input: HTMLInputElement, digitsBefore: number, text: string): void {
    let seen = 0;
    let position = text.length;
    if (digitsBefore < phoneDigits(text).length) {
      position = 0;
      while (position < text.length && seen < digitsBefore) {
        if (/\d/.test(text[position])) seen += 1;
        position += 1;
      }
    }
    input.setSelectionRange(position, position);
  }

  private scrollToActive(): void {
    queueMicrotask(() => {
      const id = this.activeId();
      if (id)
        this.panel()?.nativeElement.querySelector(`#${id}`)?.scrollIntoView?.({ block: 'nearest' });
    });
  }
}
