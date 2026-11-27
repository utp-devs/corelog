import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it } from 'vitest';

import { ProjectSettingsShell } from './project-settings-shell';
import { projectOf, settingsEnvironment } from './settings-test-kit';

async function mount(options: Parameters<typeof settingsEnvironment>[0]): Promise<HTMLElement> {
  const environment = settingsEnvironment(options);
  TestBed.configureTestingModule({ providers: [...environment.providers, provideRouter([])] });
  const fixture = TestBed.createComponent(ProjectSettingsShell);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('ProjectSettingsShell', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the seven sections for a lead with key based links', async () => {
    const element = await mount({});
    const links = Array.from(element.querySelectorAll('a')).map((link) => ({
      text: link.textContent?.trim(),
      href: link.getAttribute('href'),
    }));
    expect(links.map((link) => link.text)).toEqual([
      'Detalles',
      'Acceso',
      'Roles',
      'Permisos',
      'Tipos de incidencia',
      'Flujo de trabajo',
      'Zona de peligro',
    ]);
    expect(links[0].href).toBe('/projects/CORE/settings/details');
    expect(element.querySelector('[data-testid="settings-readonly"]')).toBeNull();
  });

  it('tells a contributor that the settings are read-only', async () => {
    const element = await mount({ role: 'contributor' });
    expect(element.querySelector('[data-testid="settings-readonly"]')?.textContent).toContain(
      'Solo lectura',
    );
    expect(element.querySelectorAll('a')).toHaveLength(7);
  });

  it('shows the forbidden state to viewers', async () => {
    const element = await mount({ role: 'viewer', permissions: ['browse_project'] });
    expect(element.textContent).toContain('No tienes acceso a la configuración');
    expect(element.querySelectorAll('a[href*="settings"]')).toHaveLength(0);
  });

  it('warns that an archived project is read-only', async () => {
    const element = await mount({ project: projectOf({ archivedAt: '2026-02-01T00:00:00Z' }) });
    expect(element.querySelector('[data-testid="settings-archived"]')?.textContent).toContain(
      'archivado',
    );
  });
});
