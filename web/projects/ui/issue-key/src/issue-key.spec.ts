import { APP_BASE_HREF } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { RouterLink, provideRouter } from '@angular/router';
import { NotificationService } from '@corelog/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { IssueKey } from './issue-key';

const notifications = { success: vi.fn(), info: vi.fn(), error: vi.fn() };

let lastFixture: ComponentFixture<IssueKey> | null = null;

/** The router link directive of the last rendered key, or null when the key is a plain link. */
function routerLinkOf(): RouterLink | null {
  return (
    lastFixture?.debugElement.query(By.directive(RouterLink))?.injector.get(RouterLink) ?? null
  );
}

function create(inputs: {
  issueKey: string;
  href?: string;
  summary?: string;
  type?: 'bug';
  resolved?: boolean;
  testId?: string;
}): HTMLElement {
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: NotificationService, useValue: notifications }],
  });
  const fixture = TestBed.createComponent(IssueKey);
  lastFixture = fixture;
  fixture.componentRef.setInput('issueKey', inputs.issueKey);
  if (inputs.href !== undefined) {
    fixture.componentRef.setInput('href', inputs.href);
  }
  if (inputs.summary !== undefined) {
    fixture.componentRef.setInput('summary', inputs.summary);
  }
  if (inputs.type !== undefined) {
    fixture.componentRef.setInput('type', inputs.type);
  }
  if (inputs.resolved !== undefined) {
    fixture.componentRef.setInput('resolved', inputs.resolved);
  }
  if (inputs.testId !== undefined) {
    fixture.componentRef.setInput('data-testid', inputs.testId);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('IssueKey', () => {
  afterEach(() => {
    Reflect.deleteProperty(navigator, 'clipboard');
    notifications.success.mockClear();
    TestBed.resetTestingModule();
  });

  it('renders the key as a link to the issue', () => {
    const el = create({ issueKey: 'CL-120', href: '/incidencias/CL-120', type: 'bug' });
    const link = el.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/incidencias/CL-120');
    expect(link?.textContent?.trim()).toBe('CL-120');
    expect(el.querySelector('[title="Error"]')).not.toBeNull();
  });

  it('navigates in-app paths through the router without a full page load', () => {
    const el = create({ issueKey: 'CL-11', href: '/incidencias/CL-11' });
    expect(el.querySelector('a')?.getAttribute('href')).toBe('/incidencias/CL-11');
    expect(routerLinkOf()).not.toBeNull();
  });

  it('keeps a plain link for absolute URLs', () => {
    const el = create({ issueKey: 'CL-12', href: 'https://example.test/incidencias/CL-12' });
    expect(el.querySelector('a')?.getAttribute('href')).toBe(
      'https://example.test/incidencias/CL-12',
    );
    expect(routerLinkOf()).toBeNull();
  });

  it('mutes resolved issues without striking the key through', () => {
    const el = create({ issueKey: 'CL-7', href: '/x', resolved: true, testId: 'key' });
    expect(el.querySelector('[data-testid="key"]')?.getAttribute('data-resolved')).toBe('true');
    expect(el.querySelector('a')?.className).toMatch(/resolved/);
  });

  it('renders plain text, not a link, when no href is given', () => {
    const el = create({ issueKey: 'INC-1234567890' });
    expect(el.querySelector('a')).toBeNull();
    expect(el.querySelector('.issue-key__text')?.textContent?.trim()).toBe('INC-1234567890');
  });

  it('shows the short form but copies the full key', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: NotificationService, useValue: notifications }],
    });
    const f = TestBed.createComponent(IssueKey);
    f.componentRef.setInput('issueKey', 'INC-1234567890');
    f.componentRef.setInput('displayKey', 'INC-123456');
    f.detectChanges();
    const el = f.nativeElement as HTMLElement;
    expect(el.querySelector('.issue-key__text')?.textContent?.trim()).toBe('INC-123456');
    el.querySelector<HTMLButtonElement>('button[aria-label="Copiar clave"]')?.click();
    await vi.waitFor(() => expect(writeText).toHaveBeenCalledWith('INC-1234567890'));
  });

  it('removes the copy button and the link from the tab order when not focusable', () => {
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: NotificationService, useValue: notifications }],
    });
    const f = TestBed.createComponent(IssueKey);
    f.componentRef.setInput('issueKey', 'CL-1');
    f.componentRef.setInput('href', '/x');
    f.componentRef.setInput('focusable', false);
    f.detectChanges();
    const el = f.nativeElement as HTMLElement;
    expect(el.querySelector('a')?.getAttribute('tabindex')).toBe('-1');
    expect(el.querySelector('button')?.getAttribute('tabindex')).toBe('-1');
  });

  it('copies the key with the copy button', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    const el = create({ issueKey: 'CL-9', href: '/x' });
    const button = el.querySelector<HTMLButtonElement>('button[aria-label="Copiar clave"]');
    button?.click();
    await vi.waitFor(() => expect(writeText).toHaveBeenCalledWith('CL-9'));
    await vi.waitFor(() => expect(notifications.success).toHaveBeenCalledWith('Clave copiada'));
  });

  it('shows the summary in a tooltip on hover', async () => {
    const el = create({ issueKey: 'CL-3', href: '/x', summary: 'Error al guardar el perfil' });
    el.querySelector('a')?.dispatchEvent(new Event('mouseenter'));
    await vi.waitFor(() => {
      const tooltip = document.querySelector('[role="tooltip"]');
      expect(tooltip?.textContent).toContain('Error al guardar el perfil');
    });
  });
});

describe('IssueKey under the workspace base href', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('prefixes the router path with the base once, not twice', () => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: APP_BASE_HREF, useValue: '/workspace/' },
        { provide: NotificationService, useValue: notifications },
      ],
    });
    const fixture = TestBed.createComponent(IssueKey);
    fixture.componentRef.setInput('issueKey', 'CL-120');
    fixture.componentRef.setInput('href', '/issues/CL-120');
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('a')?.getAttribute('href')).toBe(
      '/workspace/issues/CL-120',
    );
  });
});
