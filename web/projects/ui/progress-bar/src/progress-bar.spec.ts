import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ProgressBar } from './progress-bar';

@Component({
  imports: [ProgressBar],
  template: `<app-progress-bar [value]="value()" [label]="label()" />`,
})
class Host {
  readonly value = signal(40);
  readonly label = signal('Completado');
}

function render(value: number, label: string): HTMLElement {
  const fixture = TestBed.createComponent(Host);
  fixture.componentInstance.value.set(value);
  fixture.componentInstance.label.set(label);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('ProgressBar', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [Host] });
  });

  it('exposes the value to assistive technology with the label as name', () => {
    const el = render(40, 'Completado');
    const bar = el.querySelector('[role="progressbar"]') as HTMLElement;
    expect(bar.getAttribute('aria-label')).toBe('Completado');
    expect(bar.getAttribute('aria-valuenow')).toBe('40');
    expect(bar.getAttribute('aria-valuemin')).toBe('0');
    expect(bar.getAttribute('aria-valuemax')).toBe('100');
    expect(el.textContent).toContain('40%');
  });

  it('clamps values outside 0-100 and non-finite values', () => {
    const high = render(150, 'Alto');
    expect(high.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('100');
    const low = render(-5, 'Bajo');
    expect(low.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('0');
    const nan = render(Number.NaN, 'Nulo');
    expect(nan.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('0');
  });
});
