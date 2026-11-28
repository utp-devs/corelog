import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import {
  NotificationSchemeApiService,
  OrganizationApiService,
  TeamApiService,
  type NotificationScheme,
  type NotificationSchemeScope,
} from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { of, throwError } from 'rxjs';

import { SchemeEditor } from './scheme-editor';
import { EVENT_GROUPS } from './scheme-draft';

const EVENTS = EVENT_GROUPS.flatMap((group) => group.events.map((info) => info.event));

function scheme(overridden = false): NotificationScheme {
  return {
    scope: 'project',
    organizationId: 'o-1',
    projectId: 'p-1',
    isDefault: !overridden,
    updatedAt: null,
    rules: EVENTS.map((event) => ({
      event,
      recipients: [{ type: 'assignee' as const }],
      channels: ['in_app' as const],
      overridden,
    })),
  };
}

async function setup(
  api: Partial<NotificationSchemeApiService>,
  canEdit = true,
): Promise<ComponentFixture<SchemeEditor>> {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      { provide: NotificationSchemeApiService, useValue: api },
      { provide: OrganizationApiService, useValue: { listMembers: () => of([]) } },
      { provide: TeamApiService, useValue: { list: () => of([]) } },
    ],
  });
  const fixture = TestBed.createComponent(SchemeEditor);
  const scope: NotificationSchemeScope = { kind: 'project', id: 'p-1' };
  fixture.componentRef.setInput('scope', scope);
  fixture.componentRef.setInput('canEdit', canEdit);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return fixture;
}

function el(fixture: ComponentFixture<SchemeEditor>): HTMLElement {
  return fixture.nativeElement as HTMLElement;
}

describe('SchemeEditor', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('shows a row per event with the inherited badge and a customize action', async () => {
    const fixture = await setup({ get: vi.fn(() => of(scheme())) });
    expect(el(fixture).querySelectorAll('[data-testid^="scheme-row-"]')).toHaveLength(13);
    expect(el(fixture).querySelectorAll('[data-testid="scheme-badge-inherited"]')).toHaveLength(13);
    expect(
      el(fixture).querySelector('[data-testid="scheme-customize-issue_created"]'),
    ).not.toBeNull();
    expect(el(fixture).querySelector('[data-testid="scheme-dirty"]')).toBeNull();
  });

  it('is read-only without permission: no actions and a notice', async () => {
    const fixture = await setup({ get: vi.fn(() => of(scheme())) }, false);
    expect(el(fixture).querySelector('[data-testid="scheme-readonly"]')).not.toBeNull();
    expect(el(fixture).querySelector('[data-testid="scheme-save"]')).toBeNull();
    expect(el(fixture).querySelector('[data-testid="scheme-customize-issue_created"]')).toBeNull();
    expect(el(fixture).querySelector('.scheme-editor__chip-remove')).toBeNull();
  });

  it('customizing a row shows the unsaved banner and saves only the owned rows', async () => {
    const save = vi.fn(() => of(scheme(true)));
    const fixture = await setup({ get: vi.fn(() => of(scheme())), save });

    el(fixture)
      .querySelector<HTMLElement>('[data-testid="scheme-customize-issue_created"] button')
      ?.click();
    fixture.detectChanges();
    expect(el(fixture).querySelector('[data-testid="scheme-dirty"]')).not.toBeNull();
    expect(el(fixture).querySelector('[data-testid="scheme-badge-own"]')).not.toBeNull();

    el(fixture).querySelector<HTMLElement>('[data-testid="scheme-save"] button')?.click();
    await fixture.whenStable();
    expect(save).toHaveBeenCalledWith({ kind: 'project', id: 'p-1' }, [
      { event: 'issue_created', recipients: [{ type: 'assignee' }], channels: ['in_app'] },
    ]);
  });

  it('shows the validation message of a failed save', async () => {
    const save = vi.fn(() =>
      throwError(
        () =>
          new HttpErrorResponse({
            status: 422,
            error: { error: { code: 'notification.scheme_invalid', message: 'Esquema inválido' } },
          }),
      ),
    );
    const fixture = await setup({ get: vi.fn(() => of(scheme())), save });
    el(fixture)
      .querySelector<HTMLElement>('[data-testid="scheme-customize-issue_created"] button')
      ?.click();
    fixture.detectChanges();
    el(fixture).querySelector<HTMLElement>('[data-testid="scheme-save"] button')?.click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el(fixture).querySelector('[data-testid="scheme-error"]')?.textContent).toContain(
      'Esquema inválido',
    );
  });

  it('shows an error state with retry when the scheme cannot be read', async () => {
    const fixture = await setup({
      get: vi.fn(() => throwError(() => new HttpErrorResponse({ status: 500 }))),
    });
    expect(el(fixture).querySelector('app-error-state')).not.toBeNull();
    expect(el(fixture).querySelector('[data-testid="scheme-editor"]')).not.toBeNull();
    expect(el(fixture).querySelector('[data-testid^="scheme-row-"]')).toBeNull();
  });

  it('enables the reset only when the scope owns rules', async () => {
    const own = await setup({ get: vi.fn(() => of(scheme(true))) });
    expect(
      el(own).querySelector<HTMLButtonElement>('[data-testid="scheme-reset"] button')?.disabled,
    ).toBe(false);
    TestBed.resetTestingModule();
    const inherited = await setup({ get: vi.fn(() => of(scheme())) });
    expect(
      el(inherited).querySelector<HTMLButtonElement>('[data-testid="scheme-reset"] button')
        ?.disabled,
    ).toBe(true);
  });
});
