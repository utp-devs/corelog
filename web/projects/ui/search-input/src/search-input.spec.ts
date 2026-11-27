import { TestBed } from '@angular/core/testing';

import { SearchInput } from './search-input';

function create(value = '') {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(SearchInput);
  fixture.componentRef.setInput('value', value);
  fixture.detectChanges();
  const input = (fixture.nativeElement as HTMLElement).querySelector('input') as HTMLInputElement;
  return { fixture, input, el: fixture.nativeElement as HTMLElement };
}

function type(fixture: { detectChanges: () => void }, input: HTMLInputElement, text: string): void {
  input.value = text;
  input.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

describe('SearchInput', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('notifies only the final value after the debounce delay', () => {
    const { fixture, input } = create('');
    expect(input.getAttribute('aria-label')).toBe('Buscar');

    type(fixture, input, 'ab');
    type(fixture, input, 'abc');
    expect(fixture.componentInstance.value()).toBe('');

    vi.advanceTimersByTime(300);
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBe('abc');
  });

  it('notifies an empty value when cleared', () => {
    const { fixture, el } = create('login');
    const clear = el.querySelector('button[aria-label="Limpiar búsqueda"]') as HTMLButtonElement;
    expect(clear).not.toBeNull();
    clear.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBe('');
  });

  it('works as a ControlValueAccessor: writeValue sets the field and typing reaches onChange', () => {
    const { fixture, input } = create('');
    const onChange = vi.fn();
    fixture.componentInstance.registerOnChange(onChange);

    fixture.componentInstance.writeValue('inicial');
    fixture.detectChanges();
    expect(input.value).toBe('inicial');

    type(fixture, input, 'inicial 2');
    vi.advanceTimersByTime(300);
    expect(onChange).toHaveBeenLastCalledWith('inicial 2');
  });
});

describe('SearchInput keyboard', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('clears a non-empty field with Escape and keeps the key from bubbling', () => {
    const { fixture, input } = create('login');
    const event = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
    const stop = vi.spyOn(event, 'stopPropagation');
    input.dispatchEvent(event);
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBe('');
    expect(input.value).toBe('');
    expect(stop).toHaveBeenCalled();
  });

  it('lets Escape bubble when the field is already empty', () => {
    const { input } = create('');
    const event = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
    const stop = vi.spyOn(event, 'stopPropagation');
    input.dispatchEvent(event);
    expect(stop).not.toHaveBeenCalled();
  });

  it('gives the clear button a name and hides it while empty', () => {
    const { fixture, el } = create('');
    expect(el.querySelector('.search-input__clear')).toBeNull();
    type(fixture, el.querySelector('input') as HTMLInputElement, 'x');
    expect(el.querySelector('.search-input__clear')?.getAttribute('aria-label')).toBe(
      'Limpiar búsqueda',
    );
  });
});
