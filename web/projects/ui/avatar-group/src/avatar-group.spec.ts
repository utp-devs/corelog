import { TestBed } from '@angular/core/testing';
import { initialsOf } from '@corelog/ui/avatar';
import { describe, expect, it } from 'vitest';

import { AvatarGroup, AvatarUser } from './avatar-group';

const users: AvatarUser[] = [
  { id: '1', name: 'Ana Torres' },
  { id: '2', name: 'Luis Paredes' },
  { id: '3', name: 'Karina Navarro' },
];

function create(value: AvatarUser[], max?: number): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(AvatarGroup);
  fixture.componentRef.setInput('users', value);
  if (max !== undefined) {
    fixture.componentRef.setInput('max', max);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

function avatarTexts(root: HTMLElement): string[] {
  return Array.from(root.querySelectorAll('.avatar, .avatar-group__more')).map(
    (node) => node.getAttribute('data-initials') ?? node.textContent?.trim() ?? '',
  );
}

describe('initialsOf', () => {
  it('uses the first letter of the first two words, in uppercase', () => {
    expect(initialsOf('ana torres ruiz')).toBe('AT');
    expect(initialsOf('  pablo  ')).toBe('P');
    expect(initialsOf('')).toBe('');
  });
});

describe('AvatarGroup', () => {
  it('renders the initials of every user when all fit', () => {
    const el = create(users, 4);
    expect(avatarTexts(el)).toEqual(['AT', 'LP', 'KN']);
  });

  it('summarizes the users beyond max as a counter', () => {
    const el = create(users, 1);
    expect(avatarTexts(el)).toEqual(['AT', '+2']);
    expect(el.textContent).not.toContain('LP');
  });
});
