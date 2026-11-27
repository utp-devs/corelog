import { signal, Type } from '@angular/core';
import { ApiError, AuthStore, Project, SessionService, projectRouteResolver } from '@corelog/core';
import { Routes } from '@angular/router';
import { Observable, throwError } from 'rxjs';
import { ComponentFixture } from '@angular/core/testing';

export const PROJECT_ID = 'project-1';
export const ME_ID = 'me';

export function projectOf(): Project {
  return {
    id: PROJECT_ID,
    organizationId: 'org-1',
    key: 'CORE',
    name: 'CoreLog',
    description: '',
    type: 'kanban',
    leadId: ME_ID,
    visibility: 'open',
    archivedAt: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z',
  };
}

export function apiErrorOf(status: number): ApiError {
  return { status, code: `http.${status}`, message: 'failed', details: [] };
}

export function failWith(status: number): Observable<never> {
  return throwError(() => apiErrorOf(status));
}

/** Routes that mirror the project workspace: the planning page is a child of the `:key` route that resolves the project. */
export function projectRoutes(page: Type<unknown>, segment: string): Routes {
  return [
    {
      path: 'projects/:key',
      resolve: { project: projectRouteResolver },
      children: [{ path: segment, component: page }],
    },
  ];
}

/** Session stand-in: the store resets its data while no session is active, so tests run signed in. */
export function signedInSession(): Pick<SessionService, 'isAuthenticated'> {
  return { isAuthenticated: signal(true).asReadonly() } as unknown as Pick<
    SessionService,
    'isAuthenticated'
  >;
}

/** Auth store stand-in with a signed-in user. */
export function signedInAuth(): Pick<AuthStore, 'user'> {
  return { user: signal({ id: ME_ID, name: 'Yo' }).asReadonly() } as unknown as Pick<
    AuthStore,
    'user'
  >;
}

/** Opens a PrimeNG select and picks the option with the given label from the open overlay. */
export function pickOption(
  combobox: HTMLElement,
  label: string,
  fixture: ComponentFixture<unknown>,
): void {
  combobox.click();
  fixture.detectChanges();
  const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(
    (el) => el.textContent?.trim() === label,
  );
  if (!option) throw new Error(`option ${label} is not open`);
  option.click();
  fixture.detectChanges();
}

/** Finds an element by its aria-label anywhere in the document (overlays render outside the component). */
export function byAriaLabel(label: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[aria-label="${label}"]`);
}

/** PrimeNG overlays query window.matchMedia; jsdom does not provide it. Call once at the top of a spec. */
export function ensureMatchMedia(): void {
  if (typeof window.matchMedia === 'function') return;
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }),
  });
}
