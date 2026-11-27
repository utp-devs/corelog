import { TestBed } from '@angular/core/testing';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  DENSITY_EVENT,
  DENSITY_STORAGE_KEY,
  SIDEBAR_STORAGE_KEY,
  UiPreferencesService,
  type UiDensity,
} from './ui-preferences.service';

describe('UiPreferencesService', () => {
  beforeEach(() => {
    window.localStorage.clear();
    delete document.documentElement.dataset['density'];
  });

  it('defaults density to comfortable and reads the data table format', () => {
    expect(TestBed.inject(UiPreferencesService).density()).toBe('comfortable');
    TestBed.resetTestingModule();
    window.localStorage.setItem(DENSITY_STORAGE_KEY, '"compact"');
    expect(TestBed.inject(UiPreferencesService).density()).toBe('compact');
  });

  it('ignores unknown density values', () => {
    window.localStorage.setItem(DENSITY_STORAGE_KEY, '"huge"');
    expect(TestBed.inject(UiPreferencesService).density()).toBe('comfortable');
  });

  it('writes density as JSON and applies it to the html element', () => {
    TestBed.inject(UiPreferencesService).setDensity('compact');
    expect(window.localStorage.getItem(DENSITY_STORAGE_KEY)).toBe('"compact"');
    expect(document.documentElement.dataset['density']).toBe('compact');
  });

  it('dispatches the density event with the new value on every write', () => {
    const service = TestBed.inject(UiPreferencesService);
    const received: unknown[] = [];
    const listener = (event: Event) => received.push((event as CustomEvent<UiDensity>).detail);
    const spy = vi.fn(listener);
    window.addEventListener(DENSITY_EVENT, spy);
    service.setDensity('compact');
    service.setDensity('comfortable');
    window.removeEventListener(DENSITY_EVENT, spy);
    expect(received).toEqual(['compact', 'comfortable']);
  });

  it('keeps the sidebar state null until the user chooses', () => {
    const service = TestBed.inject(UiPreferencesService);
    expect(service.sidebarCollapsed()).toBeNull();
    service.setSidebarCollapsed(true);
    expect(service.sidebarCollapsed()).toBe(true);
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe('collapsed');
    service.setSidebarCollapsed(false);
    expect(service.sidebarCollapsed()).toBe(false);
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe('expanded');
  });
});
