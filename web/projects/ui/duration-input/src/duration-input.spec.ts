import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { Component } from '@angular/core';

import { DurationInput } from './duration-input';

@Component({
  imports: [DurationInput, ReactiveFormsModule],
  template: `<app-duration-input ariaLabel="Tiempo" [formControl]="control" />`,
})
class Host {
  readonly control = new FormControl('', { nonNullable: true });
}

function mount(): {
  el: HTMLElement;
  host: Host;
  type: (text: string) => void;
  detect: () => void;
} {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const input = el.querySelector('input') as HTMLInputElement;
  return {
    el,
    host: fixture.componentInstance,
    detect: () => fixture.detectChanges(),
    type: (text) => {
      input.value = text;
      input.dispatchEvent(new Event('input'));
      fixture.detectChanges();
    },
  };
}

describe('DurationInput', () => {
  it('writes the typed text to the form control', () => {
    const { host, type } = mount();
    type('1h 30m');
    expect(host.control.value).toBe('1h 30m');
  });

  it('shows what the text means in plain units', () => {
    const { el, type } = mount();
    type('90m');
    expect(el.querySelector('.duration-input__hint')?.textContent?.trim()).toBe('= 1h 30m');
  });

  it('explains an unreadable duration and marks the input invalid', () => {
    const { el, type } = mount();
    type('mucho');
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('semanas, días, horas');
  });

  it('rejects zero unless the field allows it and durations over the limit', () => {
    const { el, type } = mount();
    type('0m');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('mayor que cero');
    type('4d');
    expect(el.querySelector('[role="alert"]')).toBeNull();
    type('30d');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('demasiado larga');
  });

  it('shows the value written by the form', () => {
    const { el, host, detect } = mount();
    host.control.setValue('2d');
    detect();
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('2d');
  });
});
