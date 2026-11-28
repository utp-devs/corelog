import { TestBed } from '@angular/core/testing';
import {
  EMPTY_BASIC_FILTERS,
  basicFilterChips,
  basicFiltersToJql,
  nextDay,
  removeChip,
  splitOrderBy,
  withOrderBy,
} from '@corelog/core';

import { QueryChips } from './query-chips';

const filters = {
  ...EMPTY_BASIC_FILTERS,
  projects: ['KAN', 'SCRUM'],
  assignees: ['me', 'none'],
  statusCategories: ['todo'],
  text: 'cierre "rápido"',
  createdFrom: '2026-01-01',
  createdTo: '2026-01-31',
};

describe('basic filters to CoreQL', () => {
  it('joins one clause per filter', () => {
    expect(basicFiltersToJql(filters)).toBe(
      'project IN ("KAN", "SCRUM") AND statusCategory = "todo" AND (assignee = currentUser() OR assignee IS EMPTY) AND text ~ "cierre \\"rápido\\"" AND created >= "2026-01-01" AND created < "2026-02-01"',
    );
  });

  it('yields an empty text without filters', () => {
    expect(basicFiltersToJql(EMPTY_BASIC_FILTERS)).toBe('');
  });

  it('handles the end of the year when computing the next day', () => {
    expect(nextDay('2026-12-31')).toBe('2027-01-01');
  });

  it('replaces and removes the order of a query', () => {
    expect(withOrderBy('project = KAN ORDER BY created ASC', 'updated DESC')).toBe(
      'project = KAN ORDER BY updated DESC',
    );
    expect(withOrderBy('project = KAN ORDER BY created', '')).toBe('project = KAN');
    expect(splitOrderBy('text ~ "order by x" order by key')).toEqual({
      where: 'text ~ "order by x"',
      order: 'key',
    });
  });
});

describe('basic filter chips', () => {
  it('builds one chip per value and removes a single value', () => {
    const chips = basicFilterChips(filters, (_, value) => value.toUpperCase());
    expect(chips.map((chip) => chip.text)).toContain('Proyecto: KAN');
    const kan = chips.find((chip) => chip.key === 'projects:KAN');
    expect(kan).toBeDefined();
    expect(removeChip(filters, kan!).projects).toEqual(['SCRUM']);
    const from = chips.find((chip) => chip.key === 'createdFrom');
    expect(removeChip(filters, from!).createdFrom).toBe('');
  });
});

describe('QueryChips', () => {
  it('emits the removed chip and the clear request', () => {
    const fixture = TestBed.createComponent(QueryChips);
    const chips = basicFilterChips(filters);
    fixture.componentRef.setInput('chips', chips);
    fixture.detectChanges();
    const removed: string[] = [];
    let cleared = 0;
    fixture.componentInstance.remove.subscribe((chip) => removed.push(chip.key));
    fixture.componentInstance.clear.subscribe(() => (cleared += 1));
    const root = fixture.nativeElement as HTMLElement;
    root.querySelector<HTMLButtonElement>('.query-chips__remove')?.click();
    root.querySelector<HTMLButtonElement>('.query-chips__clear')?.click();
    expect(removed).toEqual([chips[0].key]);
    expect(cleared).toBe(1);
  });

  it('renders nothing without chips', () => {
    const fixture = TestBed.createComponent(QueryChips);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('ul')).toBeNull();
  });
});
