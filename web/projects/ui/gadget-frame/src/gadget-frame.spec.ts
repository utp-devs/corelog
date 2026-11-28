import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { GadgetFrame, GadgetFrameState } from './gadget-frame';

@Component({
  imports: [GadgetFrame],
  template: `
    <app-gadget-frame
      title="Asignadas a mí"
      [state]="state()"
      [subtitle]="subtitle()"
      [editing]="editing()"
      (refresh)="refreshed = refreshed + 1"
    >
      <button gadgetFrameActions type="button" id="extra">Extra</button>
      <p id="content">Contenido</p>
    </app-gadget-frame>
  `,
})
class Host {
  readonly state = signal<GadgetFrameState>('ready');
  readonly subtitle = signal<string | undefined>(undefined);
  readonly editing = signal(false);
  refreshed = 0;
}

function render() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('GadgetFrame', () => {
  it('names the region with its title and shows the content when ready', () => {
    const { el } = render();
    const section = el.querySelector('section') as HTMLElement;
    const heading = el.querySelector('h3') as HTMLElement;
    expect(heading.textContent).toContain('Asignadas a mí');
    expect(section.getAttribute('aria-labelledby')).toBe(heading.id);
    expect(el.querySelector('#content')).not.toBeNull();
    expect(el.querySelector('#extra')).not.toBeNull();
  });

  it('shows a skeleton while loading and hides the content', () => {
    const { fixture, el } = render();
    fixture.componentInstance.state.set('loading');
    fixture.detectChanges();
    expect(el.querySelector('app-loading-skeleton')).not.toBeNull();
    expect(el.querySelector('#content')).toBeNull();
    expect(el.querySelector('section')?.getAttribute('aria-busy')).toBe('true');
  });

  it('shows an alert with a retry button on error', () => {
    const { fixture, el } = render();
    fixture.componentInstance.state.set('error');
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
    (el.querySelector('.gadget-frame__retry') as HTMLButtonElement).click();
    expect(fixture.componentInstance.refreshed).toBe(1);
  });

  it('refreshes from the header button and hides it while editing', () => {
    const { fixture, el } = render();
    const button = el.querySelector('.gadget-frame__refresh') as HTMLButtonElement;
    expect(button.getAttribute('aria-label')).toBe('Actualizar Asignadas a mí');
    button.click();
    expect(fixture.componentInstance.refreshed).toBe(1);

    fixture.componentInstance.editing.set(true);
    fixture.detectChanges();
    expect(el.querySelector('.gadget-frame__refresh')).toBeNull();
    expect(el.querySelector('.gadget-frame--editing')).not.toBeNull();
  });

  it('shows the subtitle when there is one', () => {
    const { fixture, el } = render();
    fixture.componentInstance.subtitle.set('Actualizado a las 10:00');
    fixture.detectChanges();
    expect(el.textContent).toContain('Actualizado a las 10:00');
  });
});
