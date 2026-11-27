import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { ProjectAvatar } from './project-avatar';

function create(inputs: { name: string; projectKey: string; icon?: string }): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(ProjectAvatar);
  fixture.componentRef.setInput('name', inputs.name);
  fixture.componentRef.setInput('projectKey', inputs.projectKey);
  if (inputs.icon !== undefined) {
    fixture.componentRef.setInput('icon', inputs.icon);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('ProjectAvatar', () => {
  it('shows the uppercase key initials and names the project', () => {
    const el = create({ name: 'Corelog', projectKey: 'cl' });
    const tile = el.querySelector('[role="img"]');
    expect(tile?.getAttribute('aria-label')).toBe('Proyecto Corelog');
    expect(tile?.textContent).toContain('CL');
  });

  it('shows the icon instead of initials when one is given', () => {
    const el = create({ name: 'Corelog', projectKey: 'CL', icon: 'pi pi-star' });
    expect(el.querySelector('i.pi-star')).not.toBeNull();
    expect(el.querySelector('[role="img"]')?.textContent).not.toContain('CL');
  });

  it('picks the same palette slot for the same key', () => {
    const first = create({ name: 'A', projectKey: 'OPS' });
    const a = first.querySelector('[role="img"]')?.getAttribute('data-palette');
    TestBed.resetTestingModule();
    const second = create({ name: 'B', projectKey: 'OPS' });
    const b = second.querySelector('[role="img"]')?.getAttribute('data-palette');
    expect(a).toBeTruthy();
    expect(a).toBe(b);
  });

  it('renders a hostile name as text', () => {
    const el = create({ name: '<script>x</script>', projectKey: 'XS' });
    expect(el.querySelector('script')).toBeNull();
  });
});
