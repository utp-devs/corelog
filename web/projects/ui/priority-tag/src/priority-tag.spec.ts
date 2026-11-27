import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { PriorityTag } from './priority-tag';

function render(priority: { name: string; icon: string; color: string }): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(PriorityTag);
  fixture.componentRef.setInput('priority', priority);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('PriorityTag', () => {
  it('renders the name of the catalog priority', () => {
    const el = render({ name: 'Máxima', icon: 'chevrons-up', color: 'red' });
    expect(el.textContent?.trim()).toBe('Máxima');
  });

  it('renders custom priority names', () => {
    const el = render({ name: 'Bloqueante', icon: 'alert', color: 'pink' });
    expect(el.textContent?.trim()).toBe('Bloqueante');
  });

  it('draws the icon in the color of the entry and hides it from assistive technology', () => {
    const el = render({ name: 'Alta', icon: 'arrow-up', color: 'orange' });
    const icon = el.querySelector('app-catalog-icon');
    expect(icon?.getAttribute('data-color')).toBe('orange');
    expect(icon?.getAttribute('aria-hidden')).toBe('true');
  });
});
