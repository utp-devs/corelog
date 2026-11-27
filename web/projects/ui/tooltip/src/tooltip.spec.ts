import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AppTooltip, TOOLTIP_DELAY_MS } from './tooltip';

@Component({
  imports: [AppTooltip],
  template: `<button
    type="button"
    class="target"
    appTooltip="Notificaciones"
    [appTooltipDisabled]="off()"
  >
    X
  </button>`,
})
class HostComponent {
  off = signal(false);
}

function tooltipElement(): HTMLElement | null {
  return document.body.querySelector<HTMLElement>('[role="tooltip"]');
}

describe('AppTooltip', () => {
  let fixture: ComponentFixture<HostComponent>;
  let button: HTMLButtonElement;

  beforeEach(() => {
    vi.useFakeTimers();
    fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    button = fixture.debugElement.query(By.css('.target')).nativeElement;
  });

  afterEach(() => {
    vi.useRealTimers();
    fixture.destroy();
    document.body.querySelectorAll('[role="tooltip"]').forEach((node) => node.remove());
  });

  it('waits for the delay before showing on hover', () => {
    button.dispatchEvent(new Event('mouseenter'));
    vi.advanceTimersByTime(TOOLTIP_DELAY_MS - 1);
    expect(tooltipElement()).toBeNull();
    vi.advanceTimersByTime(1);
    expect(tooltipElement()?.textContent).toContain('Notificaciones');
    button.dispatchEvent(new MouseEvent('mouseleave'));
    expect(tooltipElement()).toBeNull();
  });

  it('shows on keyboard focus and hides on blur', () => {
    button.dispatchEvent(new Event('focus'));
    vi.advanceTimersByTime(TOOLTIP_DELAY_MS);
    expect(tooltipElement()).not.toBeNull();
    button.dispatchEvent(new Event('blur'));
    expect(tooltipElement()).toBeNull();
  });

  it('places the tooltip below the host by default', () => {
    button.dispatchEvent(new Event('focus'));
    vi.advanceTimersByTime(TOOLTIP_DELAY_MS);
    expect(tooltipElement()?.className).toContain('p-tooltip-bottom');
    button.dispatchEvent(new Event('blur'));
  });

  it('describes the host while visible and clears the link afterwards', () => {
    button.dispatchEvent(new Event('focus'));
    vi.advanceTimersByTime(TOOLTIP_DELAY_MS);
    const id = tooltipElement()?.id ?? '';
    expect(id).not.toBe('');
    expect(button.getAttribute('aria-describedby')).toBe(id);
    button.dispatchEvent(new Event('blur'));
    expect(button.hasAttribute('aria-describedby')).toBe(false);
  });

  it('stays hidden while disabled', () => {
    fixture.componentInstance.off.set(true);
    fixture.detectChanges();
    button.dispatchEvent(new Event('focus'));
    vi.advanceTimersByTime(TOOLTIP_DELAY_MS);
    expect(tooltipElement()).toBeNull();
    button.dispatchEvent(new Event('blur'));
  });
});
