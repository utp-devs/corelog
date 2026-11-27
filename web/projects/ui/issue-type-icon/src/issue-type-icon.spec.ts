import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { IssueTypeIcon } from './issue-type-icon';

function create(type: 'bug' | 'epic', showLabel = false): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(IssueTypeIcon);
  fixture.componentRef.setInput('type', type);
  fixture.componentRef.setInput('showLabel', showLabel);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('IssueTypeIcon', () => {
  it('exposes the type name to assistive technology', () => {
    const el = create('bug');
    expect(el.querySelector('[title="Error"]')).not.toBeNull();
    expect(el.querySelector('.issue-type-icon__hidden-label')?.textContent).toBe('Error');
  });

  it('shows the visible label when requested', () => {
    const el = create('epic', true);
    expect(el.querySelector('.issue-type-icon__hidden-label')).toBeNull();
    expect(el.textContent).toContain('Épica');
  });
});
