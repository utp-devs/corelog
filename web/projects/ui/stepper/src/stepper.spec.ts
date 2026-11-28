import { TestBed } from '@angular/core/testing';

import { Stepper } from './stepper';

describe('Stepper', () => {
  function setup(current: number) {
    const fixture = TestBed.createComponent(Stepper);
    fixture.componentRef.setInput('steps', [
      { label: 'Subir' },
      { label: 'Mapear' },
      { label: 'Fin' },
    ]);
    fixture.componentRef.setInput('current', current);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('marks the current step and offers going back to completed ones', () => {
    const { fixture, el } = setup(1);
    expect(el.querySelector('[aria-current="step"]')?.textContent).toContain('Mapear');
    const back = el.querySelector<HTMLButtonElement>('[data-testid="stepper-link-1"]');
    expect(back).not.toBeNull();
    const selected: number[] = [];
    fixture.componentInstance.select.subscribe((i) => selected.push(i));
    back?.click();
    expect(selected).toEqual([0]);
  });

  it('does not offer going back when it is disabled', () => {
    const { fixture, el } = setup(2);
    fixture.componentRef.setInput('backable', false);
    fixture.detectChanges();
    expect(el.querySelector('button')).toBeNull();
  });
});
