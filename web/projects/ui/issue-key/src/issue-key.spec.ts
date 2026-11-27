import { TestBed } from '@angular/core/testing';
import { NotificationService } from '@corelog/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { IssueKey } from './issue-key';

const notifications = { success: vi.fn(), info: vi.fn(), error: vi.fn() };

function create(inputs: {
  issueKey: string;
  href: string;
  summary?: string;
  type?: 'bug';
  resolved?: boolean;
  testId?: string;
}): HTMLElement {
  TestBed.configureTestingModule({
    providers: [{ provide: NotificationService, useValue: notifications }],
  });
  const fixture = TestBed.createComponent(IssueKey);
  fixture.componentRef.setInput('issueKey', inputs.issueKey);
  fixture.componentRef.setInput('href', inputs.href);
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

  it('marks resolved issues with a struck-through key', () => {
    const el = create({ issueKey: 'CL-7', href: '/x', resolved: true, testId: 'key' });
    expect(el.querySelector('[data-testid="key"]')?.getAttribute('data-resolved')).toBe('true');
    expect(el.querySelector('a')?.className).toMatch(/resolved/);
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
