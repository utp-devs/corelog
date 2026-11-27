import { TestBed } from '@angular/core/testing';
import { IssuePriority } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { PriorityTag } from './priority-tag';

function textFor(priority: IssuePriority): string {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(PriorityTag);
  fixture.componentRef.setInput('priority', priority);
  fixture.detectChanges();
  return (fixture.nativeElement as HTMLElement).textContent?.trim() ?? '';
}

describe('PriorityTag', () => {
  it('renders the critical priority label', () => {
    expect(textFor('critical')).toBe('Crítica');
  });

  it('renders the low priority label', () => {
    expect(textFor('low')).toBe('Baja');
  });

  it('renders the medium and high priority labels', () => {
    expect(textFor('medium')).toBe('Media');
    TestBed.resetTestingModule();
    expect(textFor('high')).toBe('Alta');
  });
});
