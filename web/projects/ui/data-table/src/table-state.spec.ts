import { TestBed } from '@angular/core/testing';

import { persistentState } from './table-state';

type Density = 'comfortable' | 'compact';

function createState<T>(key: string, initial: T) {
  return TestBed.runInInjectionContext(() => persistentState<T>(key, initial));
}

describe('usePersistentState', () => {
  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({});
  });

  it('keeps two tables with the same density key in step without a reload', () => {
    const first = createState<Density>('corelog.density', 'comfortable');
    const second = createState<Density>('corelog.density', 'comfortable');

    first.set('compact');
    expect(first.value()).toBe('compact');
    expect(second.value()).toBe('compact');
  });

  it('follows density changes made by the layout preferences', () => {
    const state = createState<Density>('corelog.density', 'comfortable');
    window.dispatchEvent(new CustomEvent('corelog:density', { detail: 'compact' }));
    expect(state.value()).toBe('compact');
  });

  it('ignores density events for other keys', () => {
    const state = createState<string>('corelog.other', 'a');
    window.dispatchEvent(new CustomEvent('corelog:density', { detail: 'compact' }));
    expect(state.value()).toBe('a');
  });
});
