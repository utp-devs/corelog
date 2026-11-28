import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { CountryStore, type Country, type PhoneValue } from '@corelog/core';

import { PhoneInput } from './phone-input';

const COUNTRIES: Country[] = [
  { code: 'PE', name: 'Perú', dialCode: '+51', example: '912345678', areas: [] },
  { code: 'CO', name: 'Colombia', dialCode: '+57', example: '3211234567', areas: [] },
  { code: 'US', name: 'Estados Unidos', dialCode: '+1', example: '2025550123', areas: [] },
  {
    code: 'DO',
    name: 'República Dominicana',
    dialCode: '+1',
    example: '8092345678',
    areas: ['809', '829', '849'],
  },
];

@Component({
  imports: [PhoneInput, ReactiveFormsModule],
  template: `<app-phone-input
    inputId="phone"
    ariaLabel="Teléfono"
    [formControl]="control"
    (e164Change)="e164 = $event"
  />`,
})
class Host {
  readonly control = new FormControl<PhoneValue | null>(null);
  e164 = '';
}

function setup(): { fixture: ComponentFixture<Host>; host: Host; el: HTMLElement } {
  const store = {
    countries: signal<readonly Country[]>(COUNTRIES),
    status: signal('ready'),
    find: (code: string | null | undefined) => COUNTRIES.find((c) => c.code === code),
    defaultCountry: () => COUNTRIES[0],
    load: () => Promise.resolve(),
  };
  TestBed.configureTestingModule({
    imports: [Host],
    providers: [{ provide: CountryStore, useValue: store }],
  });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

function field(el: HTMLElement): HTMLInputElement {
  return el.querySelector<HTMLInputElement>('input[type="tel"]') as HTMLInputElement;
}

function type(fixture: ComponentFixture<Host>, value: string): void {
  const input = field(fixture.nativeElement as HTMLElement);
  input.value = value;
  input.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

function trigger(el: HTMLElement): HTMLButtonElement {
  return el.querySelector<HTMLButtonElement>('.phone-input__country') as HTMLButtonElement;
}

describe('PhoneInput', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('preselects the default country and shows its example as the placeholder', () => {
    const { el } = setup();
    expect(trigger(el).getAttribute('aria-label')).toBe('País: Perú +51');
    expect(trigger(el).textContent).toContain('+51');
    expect(field(el).placeholder).toBe('912 345 678');
  });

  it('formats while typing and reports the national digits and the E.164 text', () => {
    const { fixture, host, el } = setup();
    type(fixture, '912345678');
    expect(field(el).value).toBe('912 345 678');
    expect(host.control.value).toEqual({ country: 'PE', number: '912345678' });
    expect(host.e164).toBe('+51912345678');
  });

  it('drops letters and caps the length of the national number', () => {
    const { fixture, host, el } = setup();
    type(fixture, '9a1b2c3d4e5f6g7h8i9j');
    expect(host.control.value?.number).toBe('9123456789');
    type(fixture, '912345678901234');
    expect(host.control.value?.number).toHaveLength(11);
    expect(field(el).value.replace(/\s/g, '').length).toBeLessThanOrEqual(11);
  });

  it('switches the country when an international number is pasted', () => {
    const { fixture, host, el } = setup();
    type(fixture, '+57 321 123 4567');
    expect(host.control.value).toEqual({ country: 'CO', number: '3211234567' });
    expect(trigger(el).getAttribute('aria-label')).toBe('País: Colombia +57');
    expect(field(el).value).toBe('321 123 4567');
  });

  it('tells the countries that share +1 apart by area code', () => {
    const { fixture, host } = setup();
    type(fixture, '+1 809 234 5678');
    expect(host.control.value?.country).toBe('DO');
    type(fixture, '+1 202 555 0123');
    expect(host.control.value?.country).toBe('US');
  });

  it('keeps a prefix that is still being typed and reports it invalid', () => {
    const { fixture, host, el } = setup();
    type(fixture, '+5');
    expect(field(el).value).toBe('+5');
    expect(host.control.value).toBeNull();
    expect(host.control.errors).toEqual({ phone: true });
  });

  it('validates the length for the country and leaves empty to the required validator', () => {
    const { fixture, host } = setup();
    expect(host.control.errors).toBeNull();
    type(fixture, '12');
    expect(host.control.errors).toEqual({ phone: true });
    type(fixture, '912345678');
    expect(host.control.errors).toBeNull();
    type(fixture, '');
    expect(host.control.value).toBeNull();
  });

  it('shows a value written by the form', () => {
    const { fixture, host, el } = setup();
    host.control.setValue({ country: 'CO', number: '3211234567' });
    fixture.detectChanges();
    expect(field(el).value).toBe('321 123 4567');
    expect(trigger(el).getAttribute('aria-label')).toBe('País: Colombia +57');
    host.control.setValue(null);
    fixture.detectChanges();
    expect(field(el).value).toBe('');
  });

  it('disables both controls with the form control', () => {
    const { fixture, host, el } = setup();
    host.control.disable();
    fixture.detectChanges();
    expect(field(el).disabled).toBe(true);
    expect(trigger(el).disabled).toBe(true);
  });

  it('marks the number invalid and ties it to its description', () => {
    TestBed.resetTestingModule();
    @Component({
      imports: [PhoneInput],
      template: `<app-phone-input
        inputId="p"
        [invalid]="true"
        describedBy="msg"
        [required]="true"
      />`,
    })
    class Invalid {}
    TestBed.configureTestingModule({
      imports: [Invalid],
      providers: [
        {
          provide: CountryStore,
          useValue: {
            countries: signal(COUNTRIES),
            status: signal('ready'),
            find: (code: string) => COUNTRIES.find((c) => c.code === code),
            defaultCountry: () => COUNTRIES[0],
            load: () => Promise.resolve(),
          },
        },
      ],
    });
    const fixture = TestBed.createComponent(Invalid);
    fixture.detectChanges();
    const input = (fixture.nativeElement as HTMLElement).querySelector('input[type="tel"]');
    expect(input?.getAttribute('aria-invalid')).toBe('true');
    expect(input?.getAttribute('aria-describedby')).toBe('msg');
    expect(input?.getAttribute('aria-required')).toBe('true');
  });
});

describe('PhoneInput country list', () => {
  afterEach(() => TestBed.resetTestingModule());

  /** Opens the panel. It lives in the overlay container, so the whole document is the search root. */
  async function open(fixture: ComponentFixture<Host>): Promise<HTMLElement> {
    trigger(fixture.nativeElement as HTMLElement).click();
    fixture.detectChanges();
    await fixture.whenStable();
    return document.body;
  }

  function search(el: HTMLElement): HTMLInputElement {
    return el.querySelector<HTMLInputElement>('input[role="combobox"]') as HTMLInputElement;
  }

  function press(el: HTMLElement, key: string): void {
    search(el).dispatchEvent(
      new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }),
    );
  }

  it('opens a listbox with every country and marks the current one selected', async () => {
    const { fixture } = setup();
    const el = await open(fixture);
    expect(trigger(el).getAttribute('aria-expanded')).toBe('true');
    const options = el.querySelectorAll('[role="option"]');
    expect(options).toHaveLength(4);
    expect(el.querySelector('[role="option"][aria-selected="true"]')?.textContent).toContain(
      'Perú',
    );
    expect(options[0].textContent).toContain('+51');
  });

  it('renders the panel through the overlay container, outside the field and its clipping ancestors', async () => {
    const { fixture } = setup();
    await open(fixture);
    const panel = document.querySelector('.phone-input__panel') as HTMLElement;
    expect(panel).not.toBeNull();
    expect(fixture.nativeElement.contains(panel)).toBe(false);
    expect(panel.closest('.cdk-overlay-container')).not.toBeNull();
    expect(panel.closest('.phone-input-overlay')).not.toBeNull();
  });

  it('opens focused on the search field and exposes the active option', async () => {
    const { fixture } = setup();
    const el = await open(fixture);
    expect(document.activeElement).toBe(search(el));
    expect(search(el).getAttribute('aria-activedescendant')).toContain('-PE');
  });

  it('selects a country with a click and closes', async () => {
    const { fixture, host } = setup();
    const el = await open(fixture);
    el.querySelectorAll<HTMLElement>('[role="option"]')[1].click();
    fixture.detectChanges();
    expect(document.querySelector('[role="listbox"]')).toBeNull();
    expect(trigger(fixture.nativeElement).getAttribute('aria-label')).toBe('País: Colombia +57');
    type(fixture, '3211234567');
    expect(host.control.value).toEqual({ country: 'CO', number: '3211234567' });
  });

  it('stays open when the press lands inside the panel and closes when the page scrolls', async () => {
    const { fixture } = setup();
    const el = await open(fixture);
    search(el).dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    fixture.detectChanges();
    expect(el.querySelector('[role="listbox"]')).not.toBeNull();
    document.body.dispatchEvent(new Event('scroll'));
    document.dispatchEvent(new Event('scroll'));
    fixture.detectChanges();
    await fixture.whenStable();
    expect(el.querySelector('[role="listbox"]')).toBeNull();
  });

  it('filters by name without accents, by prefix and by code', async () => {
    const { fixture } = setup();
    const el = await open(fixture);
    const typeSearch = (value: string) => {
      search(el).value = value;
      search(el).dispatchEvent(new Event('input'));
      fixture.detectChanges();
      return Array.from(el.querySelectorAll('[role="option"]')).map((o) => o.textContent?.trim());
    };
    expect(typeSearch('peru')[0]).toContain('Perú');
    expect(typeSearch('57')[0]).toContain('Colombia');
    expect(typeSearch('+1')).toHaveLength(2);
    expect(typeSearch('us')[0]).toContain('Estados Unidos');
    expect(typeSearch('zzz')).toEqual([]);
    expect(el.textContent).toContain('Sin resultados');
  });

  it('moves with the arrow keys, selects with Enter and focuses the number', async () => {
    const { fixture, host } = setup();
    const el = await open(fixture);
    press(el, 'ArrowDown');
    fixture.detectChanges();
    expect(search(el).getAttribute('aria-activedescendant')).toContain('-CO');
    press(el, 'Enter');
    fixture.detectChanges();
    expect(el.querySelector('[role="listbox"]')).toBeNull();
    expect(trigger(el).getAttribute('aria-label')).toBe('País: Colombia +57');
    expect(document.activeElement).toBe(field(fixture.nativeElement));
    type(fixture, '3211234567');
    expect(host.control.value).toEqual({ country: 'CO', number: '3211234567' });
  });

  it('closes with Escape and returns focus to the country button', async () => {
    const { fixture } = setup();
    document.body.appendChild(fixture.nativeElement);
    const el = await open(fixture);
    press(el, 'Escape');
    fixture.detectChanges();
    expect(el.querySelector('[role="listbox"]')).toBeNull();
    expect(document.activeElement).toBe(trigger(fixture.nativeElement));
    fixture.nativeElement.remove();
  });

  it('closes when the person presses outside', async () => {
    const { fixture } = setup();
    const el = await open(fixture);
    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    fixture.detectChanges();
    expect(el.querySelector('[role="listbox"]')).toBeNull();
  });
});
