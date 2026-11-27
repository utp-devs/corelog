import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Project, ProjectStore } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ArchivedNotice } from './archived-notice';

@Component({
  imports: [ArchivedNotice],
  template: `
    <app-archived-notice
      [archived]="archived()"
      [projectId]="'p-1'"
      [canRestore]="canRestore()"
      (restored)="restoredCount.set(restoredCount() + 1)"
    />
  `,
})
class Host {
  readonly archived = signal(true);
  readonly canRestore = signal(true);
  readonly restoredCount = signal(0);
}

function render(restore = vi.fn(() => of({ id: 'p-1' } as Project))) {
  TestBed.configureTestingModule({
    imports: [Host],
    providers: [
      provideCorelogTesting(),
      { provide: ProjectStore, useValue: { restoreProject: restore } },
    ],
  });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, restore, root: fixture.nativeElement as HTMLElement };
}

describe('ArchivedNotice', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('says the project is archived and read-only', () => {
    const { root } = render();
    expect(root.textContent).toContain('Este proyecto está archivado y es de solo lectura');
  });

  it('renders nothing when the project is not archived', () => {
    const { fixture, root } = render();
    fixture.componentInstance.archived.set(false);
    fixture.detectChanges();
    expect(root.querySelector('[data-testid="archived-notice"]')).toBeNull();
  });

  it('offers Restaurar to managers only and restores the project', () => {
    const { fixture, restore, root } = render();
    const button = root.querySelector<HTMLElement>(
      '[data-testid="archived-notice-restore"] button',
    );
    expect(button).not.toBeNull();
    button?.click();
    fixture.detectChanges();
    expect(restore).toHaveBeenCalledWith('p-1');
    expect(fixture.componentInstance.restoredCount()).toBe(1);

    fixture.componentInstance.canRestore.set(false);
    fixture.detectChanges();
    expect(root.querySelector('[data-testid="archived-notice-restore"]')).toBeNull();
  });
});
