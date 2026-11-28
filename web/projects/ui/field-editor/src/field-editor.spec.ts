import { TestBed } from '@angular/core/testing';

import { FieldEditor } from './field-editor';

function mount(inputs: Record<string, unknown>): {
  el: HTMLElement;
  emitted: unknown[];
  detect: () => void;
} {
  const fixture = TestBed.createComponent(FieldEditor);
  for (const [key, value] of Object.entries(inputs)) fixture.componentRef.setInput(key, value);
  const emitted: unknown[] = [];
  fixture.componentInstance.valueChange.subscribe((value) => emitted.push(value));
  fixture.detectChanges();
  return {
    el: fixture.nativeElement as HTMLElement,
    emitted,
    detect: () => fixture.detectChanges(),
  };
}

function blur(input: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  input.value = value;
  input.dispatchEvent(new Event('blur'));
}

describe('FieldEditor', () => {
  it('commits a text value on blur and not before', () => {
    const { el, emitted } = mount({ type: 'text', label: 'Cliente', value: null });
    const input = el.querySelector('input') as HTMLInputElement;
    input.value = 'Acme';
    input.dispatchEvent(new Event('input'));
    expect(emitted).toEqual([]);
    blur(input, 'Acme');
    expect(emitted).toEqual(['Acme']);
  });

  it('does not emit when the value did not change', () => {
    const { el, emitted } = mount({ type: 'text', label: 'Cliente', value: 'Acme' });
    blur(el.querySelector('input') as HTMLInputElement, 'Acme');
    expect(emitted).toEqual([]);
  });

  it('commits numbers as numbers', () => {
    const { el, emitted } = mount({ type: 'number', label: 'Horas', value: null });
    const input = el.querySelector('input') as HTMLInputElement;
    blur(input, '2.5');
    expect(emitted).toEqual([2.5]);
  });

  it('clears the value with null', () => {
    const { el, emitted } = mount({ type: 'text', label: 'Cliente', value: 'Acme' });
    blur(el.querySelector('input') as HTMLInputElement, '   ');
    expect(emitted).toEqual([null]);
  });

  it('splits labels by commas and drops empty parts', () => {
    const { el, emitted } = mount({ type: 'labels', label: 'Etiquetas', value: ['uno'] });
    blur(el.querySelector('input') as HTMLInputElement, 'uno, dos,, tres ');
    expect(emitted).toEqual([['uno', 'dos', 'tres']]);
  });

  it('uses a textarea for paragraphs and an accessible name on every control', () => {
    const { el } = mount({ type: 'paragraph', label: 'Notas', value: 'x' });
    const area = el.querySelector('textarea') as HTMLTextAreaElement;
    expect(area.getAttribute('aria-label')).toBe('Notas');
  });

  it('disables the control when it is read-only', () => {
    const { el } = mount({ type: 'text', label: 'Cliente', value: 'x', disabled: true });
    expect((el.querySelector('input') as HTMLInputElement).disabled).toBe(true);
  });
});
