import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router, RouterLink } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';

import { UNSAVED_TITLE, unsavedChangesGuard, warnBeforeUnload } from '@corelog/core';

import { provideUnsavedChangesDialog } from './unsaved-changes-prompt';

@Component({
  imports: [RouterLink],
  template: `
    <p>Formulario</p>
    <a routerLink="/destino">Ir al destino</a>
  `,
})
class FormScreen {
  readonly dirty = signal(true);

  constructor() {
    warnBeforeUnload(this.dirty);
  }
}

@Component({
  template: `<p>Destino</p>`,
})
class DestinoScreen {}

@Component({
  imports: [RouterLink],
  template: `
    <p>Formulario</p>
    <a routerLink="/destino">Ir al destino</a>
  `,
})
class UnguardedScreen {}

function dialogEl(): HTMLElement | null {
  return document.querySelector('[role="dialog"]');
}

function buttonIn(root: ParentNode, name: string): HTMLButtonElement | undefined {
  return Array.from(root.querySelectorAll('button')).find((b) => b.textContent?.trim() === name);
}

function clickLink(harness: RouterTestingHarness): void {
  const link = Array.from(harness.routeNativeElement!.querySelectorAll('a')).find(
    (a) => a.textContent?.trim() === 'Ir al destino',
  );
  link?.click();
}

function formText(harness: RouterTestingHarness): string {
  return harness.routeNativeElement?.textContent ?? '';
}

describe('unsavedChangesGuard', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideUnsavedChangesDialog(),
        provideRouter([
          { path: 'formulario', component: FormScreen, canDeactivate: [unsavedChangesGuard] },
          { path: 'sin-guarda', component: UnguardedScreen },
          { path: 'destino', component: DestinoScreen },
        ]),
      ],
    });
  });

  afterEach(() => {
    document.querySelectorAll('.p-dialog-mask').forEach((node) => node.remove());
    TestBed.resetTestingModule();
  });

  it('prevents the browser unload while the form is dirty and not when clean', async () => {
    const harness = await RouterTestingHarness.create('/formulario');
    const dirty = harness.routeDebugElement!.componentInstance as FormScreen;
    TestBed.flushEffects();

    const dirtyEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(dirtyEvent);
    expect(dirtyEvent.defaultPrevented).toBe(true);

    dirty.dirty.set(false);
    TestBed.flushEffects();
    const cleanEvent = new Event('beforeunload', { cancelable: true });
    window.dispatchEvent(cleanEvent);
    expect(cleanEvent.defaultPrevented).toBe(false);
  });

  it('blocks in-app navigation with a dialog when the form is dirty, and stays on "Seguir editando"', async () => {
    const harness = await RouterTestingHarness.create('/formulario');
    clickLink(harness);

    await vi.waitFor(() => expect(dialogEl()).not.toBeNull());
    expect(dialogEl()?.textContent).toContain(UNSAVED_TITLE);
    buttonIn(dialogEl()!, 'Seguir editando')?.click();

    await vi.waitFor(() => expect(dialogEl()).toBeNull());
    expect(formText(harness)).toContain('Formulario');
    expect(formText(harness)).not.toContain('Destino');
    expect(TestBed.inject(Router).url).toBe('/formulario');
  });

  it('navigates after the user confirms with "Salir"', async () => {
    const harness = await RouterTestingHarness.create('/formulario');
    clickLink(harness);

    await vi.waitFor(() => expect(dialogEl()).not.toBeNull());
    buttonIn(dialogEl()!, 'Salir')?.click();

    await vi.waitFor(() => expect(formText(harness)).toContain('Destino'));
    expect(dialogEl()).toBeNull();
  });

  it('lets navigation through without a dialog when the form is clean', async () => {
    const harness = await RouterTestingHarness.create('/formulario');
    const form = harness.routeDebugElement!.componentInstance as FormScreen;
    form.dirty.set(false);

    clickLink(harness);

    await vi.waitFor(() => expect(formText(harness)).toContain('Destino'));
    expect(dialogEl()).toBeNull();
  });

  it('does not block a route that does not register the guard: no dialog, navigation proceeds', async () => {
    const harness = await RouterTestingHarness.create('/sin-guarda');
    clickLink(harness);

    await vi.waitFor(() => expect(formText(harness)).toContain('Destino'));
    expect(dialogEl()).toBeNull();
  });
});
