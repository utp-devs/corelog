import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ColorPicker } from './color-picker';
import { isHexColor, nextPaletteIndex, paletteIndexOf } from './color-value';

function create(inputs: {
  label: string;
  value: string | null;
  allowCustom?: boolean;
}): ComponentFixture<ColorPicker> {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(ColorPicker);
  fixture.componentRef.setInput('label', inputs.label);
  fixture.componentRef.setInput('value', inputs.value);
  if (inputs.allowCustom !== undefined)
    fixture.componentRef.setInput('allowCustom', inputs.allowCustom);
  fixture.detectChanges();
  return fixture;
}

function swatch(el: HTMLElement, n: number): HTMLButtonElement {
  return el.querySelector(`button[data-palette="${n}"]`) as HTMLButtonElement;
}

function keydown(fixture: ComponentFixture<ColorPicker>, target: HTMLElement, key: string): void {
  target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  fixture.detectChanges();
}

describe('ColorPicker', () => {
  it('renders 12 palette swatches inside a named radio group', () => {
    const fixture = create({ label: 'Color de la etiqueta', value: 'palette-3' });
    const el = fixture.nativeElement as HTMLElement;
    const group = el.querySelector('[role="radiogroup"]') as HTMLElement;
    expect(group.getAttribute('aria-label')).toBe('Color de la etiqueta');
    expect(el.querySelectorAll('[role="radio"]')).toHaveLength(12);
    expect(el.querySelector('[aria-label="Color 3"]')?.getAttribute('aria-checked')).toBe('true');
  });

  it('moves the selection with the arrow keys', () => {
    const onChange = vi.fn();
    const fixture = create({ label: 'Color', value: 'palette-1' });
    fixture.componentInstance.registerOnChange(onChange);
    const el = fixture.nativeElement as HTMLElement;

    keydown(fixture, swatch(el, 1), 'ArrowRight');
    expect(onChange).toHaveBeenLastCalledWith('palette-2');
    keydown(fixture, swatch(el, 1), 'ArrowLeft');
    expect(onChange).toHaveBeenLastCalledWith('palette-12');
    keydown(fixture, swatch(el, 1), 'End');
    expect(onChange).toHaveBeenLastCalledWith('palette-12');
  });

  it('keeps only the selected swatch in the tab order', () => {
    const fixture = create({ label: 'Color', value: 'palette-5' });
    const el = fixture.nativeElement as HTMLElement;
    expect(swatch(el, 5).getAttribute('tabindex')).toBe('0');
    expect(swatch(el, 1).getAttribute('tabindex')).toBe('-1');
  });

  it('accepts a valid custom hex value', () => {
    const onChange = vi.fn();
    const fixture = create({ label: 'Color', value: null });
    fixture.componentInstance.registerOnChange(onChange);
    const input = (fixture.nativeElement as HTMLElement).querySelector(
      'input.hex-input',
    ) as HTMLInputElement;
    input.value = '#1a2B3c';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(onChange).toHaveBeenCalledWith('#1a2B3c');
    expect((fixture.nativeElement as HTMLElement).querySelector('[role="alert"]')).toBeNull();
  });

  it('shows an error and does not emit an invalid hex value', () => {
    const onChange = vi.fn();
    const fixture = create({ label: 'Color', value: null });
    fixture.componentInstance.registerOnChange(onChange);
    const el = fixture.nativeElement as HTMLElement;
    const input = el.querySelector('input.hex-input') as HTMLInputElement;
    input.value = '#12';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(onChange).not.toHaveBeenCalled();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('hexadecimal');
  });

  it('hides the custom input when allowCustom is false', () => {
    const fixture = create({ label: 'Color', value: null, allowCustom: false });
    expect((fixture.nativeElement as HTMLElement).querySelector('input.hex-input')).toBeNull();
  });
});

describe('colorValue helpers', () => {
  it('validates hex colors', () => {
    expect(isHexColor('#0A1b2C')).toBe(true);
    expect(isHexColor('#abc')).toBe(false);
    expect(isHexColor('red')).toBe(false);
  });

  it('reads palette indexes only inside 1..12', () => {
    expect(paletteIndexOf('palette-12')).toBe(12);
    expect(paletteIndexOf('palette-13')).toBeNull();
    expect(paletteIndexOf('#0A1B2C')).toBeNull();
    expect(paletteIndexOf(null)).toBeNull();
  });

  it('wraps navigation keys around the palette', () => {
    expect(nextPaletteIndex('ArrowDown', 12)).toBe(1);
    expect(nextPaletteIndex('ArrowUp', 1)).toBe(12);
    expect(nextPaletteIndex('Enter', 4)).toBeNull();
  });
});
