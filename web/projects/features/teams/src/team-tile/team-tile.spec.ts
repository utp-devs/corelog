import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { TeamTile } from './team-tile';

@Component({
  imports: [TeamTile],
  template: `<app-team-tile [id]="id" [name]="name" [color]="color" size="lg" />`,
})
class Host {
  id = 't-1';
  name = 'Soporte Nivel 1';
  color: string | null = null;
}

describe('TeamTile', () => {
  it('shows the initials and a palette color derived from the id when no color is set', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();

    const tile = fixture.nativeElement.querySelector('.team-tile') as HTMLElement;
    expect(tile.textContent?.trim()).toBe('SN');
    expect(tile.getAttribute('aria-hidden')).toBe('true');
    expect(tile.getAttribute('data-palette')).toMatch(/^([1-9]|1[0-2])$/);
    expect(tile.hasAttribute('data-custom')).toBe(false);
  });

  it('uses the custom hex color when the team has one', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.color = '#112233';
    fixture.detectChanges();

    const tile = fixture.nativeElement.querySelector('.team-tile') as HTMLElement;
    expect(tile.getAttribute('data-custom')).toBe('true');
    expect(tile.hasAttribute('data-palette')).toBe(false);
  });
});
