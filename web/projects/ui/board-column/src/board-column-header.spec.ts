import { TestBed } from '@angular/core/testing';
import { WipState } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { BoardColumnHeader } from './board-column-header';

interface Inputs {
  count: number;
  total?: number;
  wipState?: WipState;
  min?: number | null;
  max?: number | null;
  estimate?: string;
  color?: string;
}

function create(inputs: Inputs): HTMLElement {
  const fixture = TestBed.createComponent(BoardColumnHeader);
  fixture.componentRef.setInput('name', 'En curso');
  for (const [key, value] of Object.entries(inputs)) {
    fixture.componentRef.setInput(key, value);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('BoardColumnHeader', () => {
  it('shows the name, the count and the estimate', () => {
    const el = create({ count: 3, estimate: '8 pts' });
    expect(el.querySelector('.board-column-header__name')?.textContent).toContain('En curso');
    expect(el.querySelector('.board-column-header__count')?.textContent?.trim()).toBe('3');
    expect(el.querySelector('.board-column-header__estimate')?.textContent).toContain('8 pts');
  });

  it('shows the count over the limit when the column has a WIP limit', () => {
    const el = create({ count: 2, total: 4, max: 5, wipState: 'ok' });
    expect(el.querySelector('.board-column-header__count')?.textContent?.trim()).toBe('4/5');
  });

  it('warns at the limit and below the minimum without blocking', () => {
    const atMax = create({ count: 3, max: 3, wipState: 'at_max' });
    expect(atMax.querySelector('.board-column-header__count--warn')).not.toBeNull();
    const below = create({ count: 0, min: 2, wipState: 'below_min' });
    expect(below.querySelector('.board-column-header__count--warn')).not.toBeNull();
  });

  it('marks a column over its limit with an icon and the danger style, and says so in words', () => {
    const el = create({ count: 4, max: 3, wipState: 'over_max' });
    const count = el.querySelector('.board-column-header__count--over');
    expect(count).not.toBeNull();
    expect(count?.querySelector('.pi-exclamation-triangle')).not.toBeNull();
    expect(count?.getAttribute('aria-label')).toContain('Límite superado');
    expect(el.querySelector('header')?.getAttribute('data-wip-state')).toBe('over_max');
  });

  it('draws the color accent from the column color', () => {
    expect(create({ count: 0 }).querySelector('.board-column-header--blue')).toBeNull();
    expect(
      create({ count: 0, color: 'blue' }).querySelector('.board-column-header--blue'),
    ).not.toBeNull();
  });
});
