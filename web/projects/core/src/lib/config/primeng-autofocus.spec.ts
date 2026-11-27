import { Component, ChangeDetectionStrategy } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Button } from 'primeng/button';
import { afterEach, describe, expect, it } from 'vitest';

import { normalizePrimeAutofocus } from './primeng-autofocus';

@Component({
  selector: 'cl-autofocus-host',
  imports: [Button],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <p-button label="Primero" />
    <p-button label="Segundo" />
    <p-button label="Con foco" [autofocus]="true" />
  `,
})
class AutofocusHost {}

describe('normalizePrimeAutofocus', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('leaves buttons without the autofocus attribute and focuses none on first render', async () => {
    normalizePrimeAutofocus();
    normalizePrimeAutofocus();
    const fixture = TestBed.createComponent(AutofocusHost);
    document.body.appendChild(fixture.nativeElement);
    fixture.detectChanges();
    await fixture.whenStable();
    await new Promise((resolve) => setTimeout(resolve));
    fixture.detectChanges();

    const buttons = Array.from<HTMLButtonElement>(fixture.nativeElement.querySelectorAll('button'));
    expect(buttons.length).toBe(3);
    expect(buttons[0].hasAttribute('autofocus')).toBe(false);
    expect(buttons[1].hasAttribute('autofocus')).toBe(false);
    expect(buttons[2].hasAttribute('autofocus')).toBe(true);
    expect(document.activeElement).not.toBe(buttons[0]);
    expect(document.activeElement).not.toBe(buttons[1]);
    fixture.nativeElement.remove();
  });
});
