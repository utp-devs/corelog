import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { UserChip, UserChipUser } from './user-chip';

const user: UserChipUser = { id: 'u-1', name: 'Ana Pérez', role: 'Administradora' };

function create(value: UserChipUser, href?: string): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(UserChip);
  fixture.componentRef.setInput('user', value);
  if (href !== undefined) {
    fixture.componentRef.setInput('href', href);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

function findByText(root: HTMLElement, text: string): HTMLElement | undefined {
  return Array.from(root.querySelectorAll('*')).find(
    (node) => node.textContent?.trim() === text,
  ) as HTMLElement | undefined;
}

describe('UserChip', () => {
  it('shows the name and the optional role', () => {
    const el = create(user);
    expect(findByText(el, 'Ana Pérez')).toBeDefined();
    expect(findByText(el, 'Administradora')).toBeDefined();
  });

  it('renders a link only when href is given', () => {
    const linked = create(user, '/personas/u-1');
    const link = linked.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/personas/u-1');
    expect(link?.textContent).toContain('Ana Pérez');
    TestBed.resetTestingModule();
    const plain = create(user);
    expect(plain.querySelector('a')).toBeNull();
  });

  it('renders a hostile name as text', () => {
    const el = create({ id: 'u-2', name: '<script>alert(1)</script>' });
    expect(el.querySelector('script')).toBeNull();
    expect(findByText(el, '<script>alert(1)</script>')).toBeDefined();
  });
});

describe('UserChip text wrapping', () => {
  it('exposes the full name and role as title so truncated text keeps a tooltip', () => {
    const el = create(user);
    expect(findByText(el, 'Ana Pérez')?.getAttribute('title')).toBe('Ana Pérez');
    expect(findByText(el, 'Administradora')?.getAttribute('title')).toBe('Administradora');
  });
});
