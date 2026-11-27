import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { StatTile } from './stat-tile';

@Component({
  imports: [StatTile],
  template: `<app-stat-tile label="Abiertas" [value]="value()" [hint]="hint()" />`,
})
class Host {
  readonly value = signal<string | number>(12);
  readonly hint = signal<string | undefined>(undefined);
}

describe('StatTile', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [Host] });
  });

  it('renders the label and the value', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.label')?.textContent).toBe('Abiertas');
    expect(el.querySelector('.value')?.textContent).toBe('12');
  });

  it('renders the hint only when provided', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.value.set(3);
    fixture.componentInstance.hint.set('+1 esta semana');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.hint')?.textContent).toBe('+1 esta semana');

    fixture.componentInstance.hint.set(undefined);
    fixture.detectChanges();
    expect(el.textContent).not.toContain('+1 esta semana');
  });
});
