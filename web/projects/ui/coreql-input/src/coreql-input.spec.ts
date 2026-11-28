import { TestBed } from '@angular/core/testing';
import { applyCompletion, coreqlCursorContext, type CoreqlProblem } from '@corelog/core';
import { of } from 'rxjs';

import { CoreqlInput, SUGGEST_DEBOUNCE_MS, type CoreqlSuggestion } from './coreql-input';
import { errorSegments, replaceRange } from './coreql-text';

const problem = (position: number, length: number, suggestions: string[] = []): CoreqlProblem => ({
  position,
  length,
  code: 'search.unknown_field',
  message: "campo desconocido 'statu'",
  suggestions,
  expected: [],
});

describe('cursor context', () => {
  it('asks for a field at the start and after AND', () => {
    expect(coreqlCursorContext('sta', 3)).toMatchObject({ kind: 'field', prefix: 'sta', start: 0 });
    expect(coreqlCursorContext('a = 1 AND pri', 13)).toMatchObject({
      kind: 'field',
      prefix: 'pri',
    });
  });

  it('asks for an operator after a field', () => {
    expect(coreqlCursorContext('status ', 7)).toMatchObject({ kind: 'operator', field: 'status' });
  });

  it('asks for a value after an operator, inside IN lists and inside quotes', () => {
    expect(coreqlCursorContext('status = ', 9)).toMatchObject({ kind: 'value', field: 'status' });
    expect(coreqlCursorContext('project in (KAN, S', 18)).toMatchObject({
      kind: 'value',
      field: 'project',
      prefix: 'S',
    });
    expect(coreqlCursorContext('assignee = "Ana T', 17)).toMatchObject({
      kind: 'value',
      field: 'assignee',
      prefix: 'Ana T',
      inQuote: true,
    });
  });

  it('offers keywords after a complete clause and sort fields after ORDER BY', () => {
    expect(coreqlCursorContext('status = todo ', 14).kind).toBe('keyword');
    expect(coreqlCursorContext('status = todo ORDER BY ', 23).kind).toBe('order');
  });
});

describe('applying a completion', () => {
  it('replaces the typed prefix and quotes values that need it', () => {
    const text = 'assignee = Ana';
    const context = coreqlCursorContext(text, text.length);
    expect(applyCompletion(text, context, 'Ana Torres', 'value').text).toBe(
      'assignee = "Ana Torres"',
    );
  });

  it('keeps plain values unquoted and adds a space after fields', () => {
    const field = coreqlCursorContext('sta', 3);
    expect(applyCompletion('sta', field, 'status', 'field')).toEqual({
      text: 'status ',
      cursor: 7,
    });
    const value = coreqlCursorContext('status = to', 11);
    expect(applyCompletion('status = to', value, 'todo', 'value').text).toBe('status = todo');
  });
});

describe('error segments', () => {
  it('marks the range of each problem', () => {
    expect(errorSegments('statu = Done', [problem(0, 5)])).toEqual([
      { text: 'statu', error: true },
      { text: ' = Done', error: false },
    ]);
  });

  it('marks the last character for a problem without length', () => {
    expect(errorSegments('status = ', [problem(9, 0)])).toEqual([
      { text: 'status =', error: false },
      { text: ' ', error: true },
    ]);
  });

  it('replaces a range with a suggestion', () => {
    expect(replaceRange('statu = Done', 0, 5, 'status')).toEqual({
      text: 'status = Done',
      cursor: 6,
    });
  });
});

describe('CoreqlInput', () => {
  const items: CoreqlSuggestion[] = [
    { label: 'status', insert: 'status', kind: 'field', detail: 'status' },
    { label: 'start', insert: 'start', kind: 'field', detail: 'date' },
  ];

  function setup(provider = vi.fn(() => of(items))) {
    const fixture = TestBed.createComponent(CoreqlInput);
    fixture.componentRef.setInput('provider', provider);
    fixture.detectChanges();
    const area = (fixture.nativeElement as HTMLElement).querySelector(
      'textarea',
    ) as HTMLTextAreaElement;
    area.focus();
    return { fixture, area, provider };
  }

  function type(area: HTMLTextAreaElement, text: string): void {
    area.value = text;
    area.setSelectionRange(text.length, text.length);
    area.dispatchEvent(new Event('input'));
  }

  it('debounces the autocomplete and accepts a suggestion with the keyboard', () => {
    vi.useFakeTimers();
    try {
      const { fixture, area, provider } = setup();
      type(area, 's');
      type(area, 'st');
      expect(provider).not.toHaveBeenCalled();
      vi.advanceTimersByTime(SUGGEST_DEBOUNCE_MS + 1);
      fixture.detectChanges();
      expect(provider).toHaveBeenCalledTimes(1);
      const options = (fixture.nativeElement as HTMLElement).querySelectorAll('[role="option"]');
      expect(options).toHaveLength(2);

      area.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
      area.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
      fixture.detectChanges();
      expect(fixture.componentInstance.value()).toBe('start ');
    } finally {
      vi.useRealTimers();
    }
  });

  it('closes the list with Escape and runs with Ctrl+Enter', () => {
    vi.useFakeTimers();
    try {
      const { fixture, area } = setup();
      let runs = 0;
      fixture.componentInstance.run.subscribe(() => (runs += 1));
      type(area, 'st');
      vi.advanceTimersByTime(SUGGEST_DEBOUNCE_MS + 1);
      fixture.detectChanges();
      area.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      fixture.detectChanges();
      expect((fixture.nativeElement as HTMLElement).querySelector('[role="listbox"]')).toBeNull();
      area.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true }));
      expect(runs).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('lists the problems and applies the suggested fix', () => {
    const { fixture } = setup();
    fixture.componentRef.setInput('problems', [problem(0, 5, ['status'])]);
    fixture.componentInstance.value.set('statu = Done');
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="coreql-problems"]')?.textContent).toContain(
      "campo desconocido 'statu'",
    );
    root.querySelector<HTMLButtonElement>('.coreql-input__fix')?.click();
    expect(fixture.componentInstance.value()).toBe('status = Done');
  });
});
