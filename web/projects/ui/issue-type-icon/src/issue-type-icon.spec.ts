import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { IssueTypeIcon } from './issue-type-icon';

const BUG = { name: 'Error', icon: 'bug', color: 'red' };
const EPIC = { name: 'Épica', icon: 'epic', color: 'purple' };

function create(type: typeof BUG, showLabel = false): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(IssueTypeIcon);
  fixture.componentRef.setInput('type', type);
  fixture.componentRef.setInput('showLabel', showLabel);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('IssueTypeIcon', () => {
  it('exposes the type name to assistive technology', () => {
    const el = create(BUG);
    expect(el.querySelector('[title="Error"]')).not.toBeNull();
    expect(el.querySelector('.issue-type-icon__hidden-label')?.textContent).toBe('Error');
  });

  it('shows the visible label when requested', () => {
    const el = create(EPIC, true);
    expect(el.querySelector('.issue-type-icon__hidden-label')).toBeNull();
    expect(el.textContent).toContain('Épica');
  });

  it('draws the glyph in the color of the catalog entry', () => {
    const el = create(EPIC);
    expect(el.querySelector('app-catalog-icon')?.getAttribute('data-color')).toBe('purple');
    expect(el.querySelectorAll('svg path').length).toBeGreaterThan(0);
  });

  it('draws a fallback glyph for an unknown icon key', () => {
    const el = create({ name: 'Raro', icon: 'does-not-exist', color: 'nope' });
    expect(el.querySelectorAll('svg path').length).toBe(1);
  });
});
