import { Provider, Type } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ProjectSettingsContext } from './project-settings-context';
import { SettingsEnvironment } from './settings-test-kit';

export function buttonNamed(name: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === name,
  );
}

export function buttonLabelled(label: string): HTMLButtonElement | null {
  return document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
}

/** Creates a settings page with its context and waits until the first reads have landed. */
export async function mountSettingsPage<T>(
  page: Type<T>,
  environment: SettingsEnvironment,
  extra: Provider[] = [],
): Promise<ComponentFixture<T>> {
  TestBed.configureTestingModule({
    providers: [...environment.providers, ProjectSettingsContext, ...extra],
  });
  const fixture = TestBed.createComponent(page);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return fixture;
}
