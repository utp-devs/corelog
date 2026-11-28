import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { Avatar } from './avatar';
import { avatarColorIndex, hashString } from './avatar-color';

function create(inputs: { id: string; name: string; src?: string | null }): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(Avatar);
  fixture.componentRef.setInput('id', inputs.id);
  fixture.componentRef.setInput('name', inputs.name);
  if (inputs.src !== undefined) {
    fixture.componentRef.setInput('src', inputs.src);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('Avatar', () => {
  it('shows the initials and uses the full name as accessible name', () => {
    const el = create({ id: 'u1', name: 'Ana Pérez' });
    const avatar = el.querySelector('[role="img"]');
    expect(avatar?.getAttribute('aria-label')).toBe('Ana Pérez');
    expect(avatar?.getAttribute('data-initials')).toBe('AP');
  });

  it('renders the photo with the name as alt text when src is given', () => {
    const el = create({ id: 'u1', name: 'Ana Pérez', src: '/media/ana.png' });
    const img = el.querySelector('img');
    expect(img).not.toBeNull();
    expect(img?.getAttribute('alt')).toBe('Ana Pérez');
    expect(img?.getAttribute('src')).toBe('/media/ana.png');
  });

  it('falls back to the initials when the photo fails to load', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(Avatar);
    fixture.componentRef.setInput('id', 'u1');
    fixture.componentRef.setInput('name', 'Ana Pérez');
    fixture.componentRef.setInput('src', '/media/broken.png');
    fixture.detectChanges();
    const img = (fixture.nativeElement as HTMLElement).querySelector('img');
    img?.dispatchEvent(new Event('error'));
    fixture.detectChanges();
    const avatar = (fixture.nativeElement as HTMLElement).querySelector('[role="img"]');
    expect(avatar?.getAttribute('data-initials')).toBe('AP');
    expect((fixture.nativeElement as HTMLElement).querySelector('img')).toBeNull();
  });

  it('gives the same color class for the same id', () => {
    const first = create({ id: 'user-42', name: 'Ana' });
    const firstClass = first.querySelector('.avatar')?.className;
    TestBed.resetTestingModule();
    const second = create({ id: 'user-42', name: 'Otra Persona' });
    const secondClass = second.querySelector('.avatar')?.className;
    expect(firstClass).toBeTruthy();
    expect(firstClass).toBe(secondClass);
  });

  it('maps ids to one of the ten avatar colors and spreads them over several', () => {
    const indexes = new Set<number>();
    for (let i = 0; i < 200; i += 1) {
      const index = avatarColorIndex(`user-${i}`);
      expect(index).toBeGreaterThanOrEqual(1);
      expect(index).toBeLessThanOrEqual(10);
      indexes.add(index);
    }
    expect(indexes.size).toBeGreaterThanOrEqual(8);
  });

  it('hashes deterministically', () => {
    expect(hashString('abc')).toBe(hashString('abc'));
    expect(hashString('abc')).not.toBe(hashString('abd'));
  });

  it('renders hostile names as text, never as markup', () => {
    const name = '<script>alert(1)</script>';
    const el = create({ id: 'u1', name });
    expect(el.querySelector('script')).toBeNull();
    expect(el.querySelector('[role="img"]')?.getAttribute('aria-label')).toBe(name);
  });
});
