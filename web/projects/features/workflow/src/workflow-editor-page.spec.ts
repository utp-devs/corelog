import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CurrentProjectStore, OrganizationStore, WorkflowStore } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { WorkflowEditorPage } from './workflow-editor-page';

if (!window.matchMedia) {
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

const WORKFLOW_URL = '/api/v1/projects/p1/workflow';
const VIEW = {
  statuses: [
    { id: 's1', name: 'Por hacer', category: 'todo', position: 0, issue_count: 2 },
    { id: 's2', name: 'Hecho', category: 'done', position: 1, issue_count: 0 },
  ],
  initial_status_id: 's1',
  transitions: [
    {
      id: 'finish',
      name: 'Terminar',
      from: ['s1'],
      to: 's2',
      conditions: [],
      validators: [],
      post_functions: [],
      required_fields: [],
    },
  ],
  version: 2,
  updated_at: '2026-11-27T00:00:00Z',
  is_template_default: false,
};

function mount(canManage: boolean): {
  fixture: ComponentFixture<WorkflowEditorPage>;
  http: HttpTestingController;
} {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  const current = TestBed.inject(CurrentProjectStore);
  vi.spyOn(current, 'projectId').mockReturnValue('p1');
  vi.spyOn(current, 'can').mockImplementation(
    (permission) => canManage && permission === 'manage_workflow',
  );
  vi.spyOn(current, 'isArchived').mockReturnValue(false);
  const orgs = TestBed.inject(OrganizationStore);
  vi.spyOn(orgs, 'currentOrgId').mockReturnValue(null);
  const http = TestBed.inject(HttpTestingController);
  const fixture = TestBed.createComponent(WorkflowEditorPage);
  fixture.detectChanges();
  http.expectOne(WORKFLOW_URL).flush({ data: VIEW });
  fixture.detectChanges();
  return { fixture, http };
}

function click(testId: string): void {
  const target = document.querySelector(`[data-testid="${testId}"]`) as HTMLElement;
  (target.tagName === 'BUTTON' ? target : (target.querySelector('button') as HTMLElement)).click();
}

describe('WorkflowEditorPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the statuses and the transitions of the workflow', () => {
    mount(true);
    expect(document.querySelectorAll('[data-testid="workflow-status"]')).toHaveLength(2);
    expect(document.querySelectorAll('[data-testid="workflow-transition"]')).toHaveLength(1);
    expect(document.querySelector('[data-testid="workflow-initial-badge"]')).not.toBeNull();
  });

  it('is read-only without the manage permission', () => {
    mount(false);
    expect(document.querySelector('[data-testid="workflow-readonly"]')).not.toBeNull();
    expect(document.querySelector('[data-testid="workflow-add-status"]')).toBeNull();
    expect(document.querySelector('[data-testid="workflow-templates"]')).toBeNull();
    expect(document.querySelector('[data-testid="workflow-add-transition"]')).toBeNull();
  });

  it('shows the dirty banner after an edit and saves with the loaded version', () => {
    const { fixture, http } = mount(true);
    expect(document.querySelector('[data-testid="workflow-dirty"]')).toBeNull();
    click('workflow-add-status');
    fixture.detectChanges();
    const name = document.querySelector(
      '[data-testid="workflow-new-status-name"]',
    ) as HTMLInputElement;
    name.value = 'Bloqueado';
    name.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
    click('workflow-add-status-confirm');
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="workflow-dirty"]')).not.toBeNull();
    expect(TestBed.inject(WorkflowStore).dirty()).toBe(true);

    click('workflow-save');
    const request = http.expectOne(WORKFLOW_URL);
    expect(request.request.method).toBe('PUT');
    expect(request.request.body.expected_version).toBe(2);
    expect(request.request.body.statuses[2].name).toBe('Bloqueado');
    request.flush({ data: { ...VIEW, version: 3 } });
    fixture.detectChanges();
    expect(TestBed.inject(WorkflowStore).dirty()).toBe(false);
  });

  it('lists the validation errors by transition name and validates without saving', () => {
    const { fixture, http } = mount(true);
    click('workflow-add-transition');
    fixture.detectChanges();
    click('workflow-validate');
    const request = http.expectOne(`${WORKFLOW_URL}/validate`);
    request.flush({
      data: {
        valid: false,
        errors: [
          {
            path: 'transitions[1].to',
            code: 'workflow.unknown_status',
            message: 'Estado desconocido',
          },
        ],
        warnings: [],
        mapping_required: [],
      },
    });
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="workflow-validation"]')?.textContent).toContain(
      'Transición «Nueva transición»: Estado desconocido',
    );
  });

  it('asks where the issues go before removing a status that holds issues', () => {
    const { fixture } = mount(true);
    (
      document.querySelector('button[aria-label="Eliminar Por hacer"]') as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="workflow-mapping-dialog"]')).not.toBeNull();
    // Nothing is removed until a target is chosen.
    expect(document.querySelectorAll('[data-testid="workflow-status"]')).toHaveLength(2);
  });

  it('offers a reload instead of overwriting when the version is stale', () => {
    const { fixture, http } = mount(true);
    click('workflow-add-transition');
    fixture.detectChanges();
    click('workflow-save');
    http
      .expectOne(WORKFLOW_URL)
      .flush(
        { error: { code: 'workflow.version_conflict', message: 'cambió' } },
        { status: 409, statusText: 'Conflict' },
      );
    fixture.detectChanges();
    expect(document.querySelector('[data-testid="workflow-conflict"]')).not.toBeNull();
    // The draft is kept until the person chooses to reload.
    expect(TestBed.inject(WorkflowStore).dirty()).toBe(true);
  });

  it('exposes the dirty state for the leave guard', () => {
    const { fixture } = mount(true);
    expect(fixture.componentInstance.dirty()).toBe(false);
    click('workflow-add-transition');
    fixture.detectChanges();
    expect(fixture.componentInstance.dirty()).toBe(true);
  });
});
