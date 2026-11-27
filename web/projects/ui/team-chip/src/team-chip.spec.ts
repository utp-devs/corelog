import { APP_BASE_HREF } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it } from 'vitest';

import { TeamChip } from './team-chip';

function create(inputs: {
  id: string;
  name: string;
  memberCount?: number;
  href?: string;
}): HTMLElement {
  TestBed.configureTestingModule({ providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(TeamChip);
  fixture.componentRef.setInput('id', inputs.id);
  fixture.componentRef.setInput('name', inputs.name);
  if (inputs.memberCount !== undefined) {
    fixture.componentRef.setInput('memberCount', inputs.memberCount);
  }
  if (inputs.href !== undefined) {
    fixture.componentRef.setInput('href', inputs.href);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

function findByText(root: HTMLElement, text: string): HTMLElement | undefined {
  return Array.from(root.querySelectorAll('*')).find(
    (node) => node.textContent?.trim() === text,
  ) as HTMLElement | undefined;
}

describe('TeamChip', () => {
  it('shows the name with the plural member count', () => {
    const el = create({ id: 't-1', name: 'Plataforma', memberCount: 4 });
    expect(findByText(el, 'Plataforma')).toBeDefined();
    expect(findByText(el, '4 miembros')).toBeDefined();
  });

  it('uses the singular for one member and hides the count when absent', () => {
    const single = create({ id: 't-2', name: 'Soporte', memberCount: 1 });
    expect(findByText(single, '1 miembro')).toBeDefined();
    TestBed.resetTestingModule();
    const none = create({ id: 't-3', name: 'Soporte' });
    expect(none.textContent).not.toMatch(/miembro/);
  });

  it('renders a link only when href is given', () => {
    const el = create({ id: 't-4', name: 'Diseño', href: '/equipos/t-4' });
    const link = el.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/equipos/t-4');
    expect(link?.textContent).toContain('Diseño');
  });

  it('renders a hostile name as text', () => {
    const el = create({ id: 't-5', name: '<img src=x onerror=alert(1)>' });
    expect(el.querySelector('img')).toBeNull();
  });
});

describe('TeamChip text wrapping', () => {
  it('exposes the full team name and member count as title so truncated text keeps a tooltip', () => {
    const el = create({ id: 't-1', name: 'Plataforma de datos internos', memberCount: 3 });
    expect(findByText(el, 'Plataforma de datos internos')?.getAttribute('title')).toBe(
      'Plataforma de datos internos',
    );
    expect(findByText(el, '3 miembros')?.getAttribute('title')).toBe('3 miembros');
  });
});

describe('TeamChip under the workspace base href', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('links in-app team paths through the router with the base prefixed once', () => {
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: APP_BASE_HREF, useValue: '/workspace/' }],
    });
    const fixture = TestBed.createComponent(TeamChip);
    fixture.componentRef.setInput('id', 't1');
    fixture.componentRef.setInput('name', 'Plataforma');
    fixture.componentRef.setInput('href', '/teams/t1');
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('a')?.getAttribute('href')).toBe(
      '/workspace/teams/t1',
    );
  });

  it('keeps absolute URLs as plain links', () => {
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: APP_BASE_HREF, useValue: '/workspace/' }],
    });
    const fixture = TestBed.createComponent(TeamChip);
    fixture.componentRef.setInput('id', 't1');
    fixture.componentRef.setInput('name', 'Plataforma');
    fixture.componentRef.setInput('href', 'https://example.test/teams/t1');
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('a')?.getAttribute('href')).toBe(
      'https://example.test/teams/t1',
    );
  });
});
