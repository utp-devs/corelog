import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { Section, SectionActions, SectionFooter, SectionHeadingLevel } from './section';

@Component({
  imports: [Section, SectionActions, SectionFooter],
  template: `
    <app-section
      [title]="title()"
      [description]="description()"
      [padded]="padded()"
      [flush]="flush()"
      [fill]="fill()"
      [headingLevel]="level()"
      [ariaLabel]="ariaLabel()"
    >
      @if (withActions()) {
        <button appSectionActions type="button">Invitar</button>
      }
      @if (withFooter()) {
        <footer appSectionFooter><span>Total: 3</span></footer>
      }
      x
    </app-section>
  `,
})
class Host {
  readonly title = signal<string | undefined>(undefined);
  readonly description = signal<string | undefined>(undefined);
  readonly padded = signal(true);
  readonly flush = signal(false);
  readonly fill = signal(false);
  readonly level = signal<SectionHeadingLevel>(2);
  readonly ariaLabel = signal<string | undefined>(undefined);
  readonly withActions = signal(false);
  readonly withFooter = signal(false);
}

function create(): { fixture: ComponentFixture<Host>; el: HTMLElement } {
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('Section', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [Host] });
  });

  it('renders the title as a level 2 heading that names the region', () => {
    const { fixture, el } = create();
    fixture.componentInstance.title.set('Detalles');
    fixture.detectChanges();
    const region = el.querySelector('section') as HTMLElement;
    const heading = el.querySelector('h2') as HTMLElement;
    expect(heading.textContent?.trim()).toBe('Detalles');
    expect(region.getAttribute('aria-labelledby')).toBe(heading.id);
    expect(region.getAttribute('aria-labelledby')).not.toBe('');
  });

  it('honours headingLevel', () => {
    const { fixture, el } = create();
    fixture.componentInstance.title.set('Subbloque');
    fixture.componentInstance.level.set(4);
    fixture.detectChanges();
    expect(el.querySelector('h4')?.textContent?.trim()).toBe('Subbloque');
    expect(el.querySelector('h2')).toBeNull();
  });

  it('renders description, actions and footer when provided', () => {
    const { fixture, el } = create();
    fixture.componentInstance.title.set('Miembros');
    fixture.componentInstance.description.set('Personas con acceso a la organización');
    fixture.componentInstance.withActions.set(true);
    fixture.componentInstance.withFooter.set(true);
    fixture.detectChanges();
    expect(el.textContent).toContain('Personas con acceso a la organización');
    expect(el.querySelector('button')?.textContent?.trim()).toBe('Invitar');
    expect(el.querySelector('footer')?.textContent).toContain('Total: 3');
  });

  it('uses ariaLabel as the region name when there is no title', () => {
    const { fixture, el } = create();
    fixture.componentInstance.ariaLabel.set('Resumen');
    fixture.detectChanges();
    expect(el.querySelector('section')?.getAttribute('aria-label')).toBe('Resumen');
    expect(el.querySelector('h2')).toBeNull();
  });

  it('omits the header and footer when not provided', () => {
    const { el } = create();
    expect(el.querySelector('header')).toBeNull();
    expect(el.querySelector('footer')).toBeNull();
  });

  it('pads the body by default and removes the padding with padded=false or flush', () => {
    const { fixture, el } = create();
    const body = () => el.querySelector('section > div') as HTMLElement;
    expect(body().classList).toContain('section__body');
    expect(body().classList).not.toContain('section__body--flush');

    fixture.componentInstance.padded.set(false);
    fixture.detectChanges();
    expect(body().classList).toContain('section__body--flush');

    fixture.componentInstance.padded.set(true);
    fixture.componentInstance.flush.set(true);
    fixture.detectChanges();
    expect(body().classList).toContain('section__body--flush');
  });

  it('is not filled by default', () => {
    const { el } = create();
    const section = el.querySelector('section') as HTMLElement;
    expect(section.getAttribute('data-fill')).toBe('false');
    expect((section.lastElementChild as HTMLElement).classList).not.toContain(
      'section__body--grow',
    );
  });

  it('grows with fill and stretches its body to the remaining height', () => {
    const { fixture, el } = create();
    fixture.componentInstance.title.set('Miembros');
    fixture.componentInstance.fill.set(true);
    fixture.detectChanges();
    const section = el.querySelector('section') as HTMLElement;
    expect(section.getAttribute('data-fill')).toBe('true');
    expect((section.querySelector('.section__body') as HTMLElement).classList).toContain(
      'section__body--grow',
    );
  });
});
