import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  CalendarV2ApiService,
  CurrentProjectStore,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  ProjectStore,
} from '@corelog/core';
import { of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ProjectCalendarPage } from './project-calendar-page';

function setup(projectId: string) {
  const window = vi.fn(() => of({ items: [], truncated: false }));
  TestBed.configureTestingModule({
    imports: [ProjectCalendarPage],
    providers: [
      provideRouter([]),
      { provide: CurrentProjectStore, useValue: { projectId: signal(projectId) } },
      { provide: CalendarV2ApiService, useValue: { window } },
      { provide: ProjectStore, useValue: { fetchProjects: vi.fn(), projects: signal([]) } },
      { provide: OrganizationApiService, useValue: { listMembers: () => of([]) } },
      { provide: ORGANIZATION_ID_PROVIDER, useValue: () => 'org-1' },
    ],
  });
  const fixture = TestBed.createComponent(ProjectCalendarPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, window };
}

describe('ProjectCalendarPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('limits the calendar to the project and hides the project filter', async () => {
    const { fixture, el, window } = setup('p-1');
    await fixture.whenStable();
    fixture.detectChanges();
    const calls = window.mock.calls as unknown as [unknown, { projects?: string[] }][];
    expect(calls[calls.length - 1][1].projects).toEqual(['p-1']);
    expect(el.querySelector('[data-testid="calendar-view"]')).not.toBeNull();
    expect(el.querySelector('p-multiselect')).toBeNull();
    expect(el.textContent).toContain('Calendario');
  });

  it('waits for the project before drawing the calendar', () => {
    const { el, window } = setup('');
    expect(el.querySelector('[data-testid="calendar-view"]')).toBeNull();
    expect(window).not.toHaveBeenCalled();
  });
});
