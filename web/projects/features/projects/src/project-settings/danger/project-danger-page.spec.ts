import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { throwError } from 'rxjs';
import { afterEach, describe, expect, it } from 'vitest';

import { buttonNamed, mountSettingsPage } from '../settings-spec-helpers';
import { projectOf, settingsEnvironment } from '../settings-test-kit';
import { ProjectDangerPage } from './project-danger-page';

function mount(options: Parameters<typeof settingsEnvironment>[0] = {}) {
  const environment = settingsEnvironment(options);
  const router = { provide: Router, useValue: { navigateByUrl: environment.navigateByUrl } };
  return { environment, mounted: mountSettingsPage(ProjectDangerPage, environment, [router]) };
}

function press(testId: string): void {
  document.querySelector<HTMLButtonElement>(`[data-testid="${testId}"] button`)?.click();
}

describe('ProjectDangerPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('archives after a confirmation that explains the effect', async () => {
    const { environment, mounted } = mount();
    const fixture = await mounted;
    press('danger-archive');
    fixture.detectChanges();
    expect(document.body.textContent).toContain('solo lectura');
    expect(environment.projectApi['archive']).not.toHaveBeenCalled();
    buttonNamed('Archivar')?.click();
    expect(environment.projectApi['archive']).toHaveBeenCalledWith('p-1');
    expect(environment.current.setProject).toHaveBeenCalled();
  });

  it('offers to restore an archived project', async () => {
    const { environment, mounted } = mount({
      project: projectOf({ archivedAt: '2026-02-01T00:00:00Z' }),
    });
    const fixture = await mounted;
    expect(document.body.textContent).toContain('Restaurar el proyecto');
    press('danger-archive');
    fixture.detectChanges();
    buttonNamed('Restaurar')?.click();
    expect(environment.projectApi['restore']).toHaveBeenCalledWith('p-1');
  });

  it('deletes only when the exact name is typed and then goes back to the project list', async () => {
    const { environment, mounted } = mount();
    const fixture = await mounted;
    press('danger-delete');
    fixture.detectChanges();
    await fixture.whenStable();

    const input = document.querySelector<HTMLInputElement>('app-confirm-danger-modal input')!;
    const submit = (): HTMLButtonElement =>
      Array.from(document.querySelectorAll('app-confirm-danger-modal button')).find(
        (button) => button.textContent?.trim() === 'Eliminar proyecto',
      ) as HTMLButtonElement;
    input.value = 'corelog';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(submit().disabled).toBe(true);

    input.value = 'CoreLog';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(submit().disabled).toBe(false);
    submit().click();
    expect(environment.projectApi['remove']).toHaveBeenCalledWith('p-1', 'CoreLog');
    expect(environment.navigateByUrl).toHaveBeenCalledWith('/projects');
  });

  it('shows a failure in Spanish', async () => {
    const { mounted } = mount({
      projectApi: {
        archive: () =>
          throwError(() => ({
            status: 409,
            code: 'project.already_archived',
            message: 'x',
            details: [],
          })),
      },
    });
    const fixture = await mounted;
    press('danger-archive');
    fixture.detectChanges();
    buttonNamed('Archivar')?.click();
    fixture.detectChanges();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('ya estaba archivado');
  });

  it('hides the actions from people who cannot use them', async () => {
    const { mounted } = mount({ role: 'contributor' });
    await mounted;
    expect(document.querySelector('[data-testid="danger-archive"]')).toBeNull();
    expect(document.querySelector('[data-testid="danger-delete"]')).toBeNull();
    expect(document.body.textContent).toContain('Solo los líderes pueden archivar');
  });
});
