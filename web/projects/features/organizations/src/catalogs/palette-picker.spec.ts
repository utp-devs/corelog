import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it } from 'vitest';

import { PalettePicker } from './palette-picker';

function mount(kind: 'icon' | 'color', value: string | null) {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(PalettePicker);
  fixture.componentRef.setInput('kind', kind);
  fixture.componentRef.setInput('label', kind === 'icon' ? 'Icono' : 'Color');
  fixture.componentRef.setInput('value', value);
  fixture.detectChanges();
  const root = fixture.nativeElement as HTMLElement;
  const radios = () => Array.from(root.querySelectorAll<HTMLButtonElement>('[role="radio"]'));
  return { fixture, root, radios };
}

describe('PalettePicker', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers the ten colors and the twenty-two icons as a radio group', () => {
    expect(mount('color', null).radios()).toHaveLength(10);
    TestBed.resetTestingModule();
    const icons = mount('icon', 'bug');
    expect(icons.radios()).toHaveLength(22);
    expect(icons.root.querySelector('[role="radiogroup"]')?.getAttribute('aria-label')).toBe(
      'Icono',
    );
  });

  it('marks the chosen option and keeps a single tab stop on it', () => {
    const { radios } = mount('color', 'green');
    const checked = radios().filter((radio) => radio.getAttribute('aria-checked') === 'true');
    expect(checked.map((radio) => radio.getAttribute('aria-label'))).toEqual(['Verde']);
    expect(radios().filter((radio) => radio.tabIndex === 0)).toHaveLength(1);
    expect(checked[0].tabIndex).toBe(0);
  });

  it('chooses with a click and with the arrow keys, wrapping at the ends', () => {
    const { fixture, radios } = mount('color', 'gray');
    radios()[3].click();
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBe('amber');

    radios()[3].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBe('green');

    radios()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBe('pink');
  });

  it('tints the icons with the given color', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(PalettePicker);
    fixture.componentRef.setInput('kind', 'icon');
    fixture.componentRef.setInput('label', 'Icono');
    fixture.componentRef.setInput('tint', 'red');
    fixture.detectChanges();
    const icon = (fixture.nativeElement as HTMLElement).querySelector('app-catalog-icon');
    expect(icon?.getAttribute('data-color')).toBe('red');
  });
});
