import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';

import { HotkeyBinding, HotkeysService, SEQUENCE_TIMEOUT_MS } from './hotkeys.service';

function press(key: string, init: KeyboardEventInit = {}, target: EventTarget = window): void {
  target.dispatchEvent(
    new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init }),
  );
}

function setup(): HotkeysService {
  TestBed.configureTestingModule({ providers: [provideRouter([{ path: '**', children: [] }])] });
  return TestBed.inject(HotkeysService);
}

describe('HotkeysService', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
  });

  it('fires a single key binding', () => {
    const hotkeys = setup();
    const run = vi.fn();
    hotkeys.register([{ keys: '?', description: 'Ayuda', group: 'General', run }]);
    press('?');
    expect(run).toHaveBeenCalledTimes(1);
  });

  it('fires a sequence when the second key arrives in time', () => {
    const hotkeys = setup();
    const run = vi.fn();
    hotkeys.register([{ keys: 'g b', description: 'Tablero', group: 'Nav', run }]);
    press('g');
    vi.advanceTimersByTime(SEQUENCE_TIMEOUT_MS - 100);
    press('b');
    expect(run).toHaveBeenCalledTimes(1);
  });

  it('drops a sequence after the timeout', () => {
    const hotkeys = setup();
    const run = vi.fn();
    hotkeys.register([{ keys: 'g b', description: 'Tablero', group: 'Nav', run }]);
    press('g');
    vi.advanceTimersByTime(SEQUENCE_TIMEOUT_MS + 1);
    press('b');
    expect(run).not.toHaveBeenCalled();
  });

  it('ignores keys typed inside inputs, textareas and contenteditable elements', () => {
    const hotkeys = setup();
    const run = vi.fn();
    hotkeys.register([{ keys: 'c', description: 'Crear', group: 'General', run }]);
    const input = document.createElement('input');
    const textarea = document.createElement('textarea');
    const editable = document.createElement('div');
    editable.setAttribute('contenteditable', 'true');
    document.body.append(input, textarea, editable);
    press('c', {}, input);
    press('c', {}, textarea);
    press('c', {}, editable);
    expect(run).not.toHaveBeenCalled();
    input.remove();
    textarea.remove();
    editable.remove();
  });

  it('ignores bindings with Ctrl, Meta or Alt', () => {
    const hotkeys = setup();
    const run = vi.fn();
    hotkeys.register([{ keys: 'c', description: 'Crear', group: 'General', run }]);
    press('c', { ctrlKey: true });
    press('c', { metaKey: true });
    press('c', { altKey: true });
    expect(run).not.toHaveBeenCalled();
  });

  it('never fires coming soon bindings but lists them', () => {
    const hotkeys = setup();
    const run = vi.fn();
    hotkeys.register([
      { keys: 'n', description: 'Notas', group: 'General', comingSoon: true, run },
    ]);
    press('n');
    expect(run).not.toHaveBeenCalled();
    expect(hotkeys.registered()).toContainEqual(
      expect.objectContaining({ keys: 'n', comingSoon: true }),
    );
  });

  it('leaves the work-page shortcuts out in an app that does not mount them', () => {
    const hotkeys = setup();
    const unregister = hotkeys.registerGlobalShortcuts({ workPages: false });
    const keys = hotkeys.registered().map((binding) => binding.keys);
    expect(keys).toEqual(expect.arrayContaining(['?', 'n']));
    expect(keys).not.toContain('c');
    expect(keys).not.toContain('g i');
    expect(keys).not.toContain('g b');
    unregister();
  });

  it('removes its bindings from the registry when unregistered', () => {
    const hotkeys = setup();
    const bindings: HotkeyBinding[] = [
      { keys: 'c', description: 'Crear incidencia', group: 'General', run: () => undefined },
    ];
    const unregister = hotkeys.register(bindings);
    expect(hotkeys.registered().map((b) => b.description)).toContain('Crear incidencia');
    unregister();
    expect(hotkeys.registered().map((b) => b.description)).not.toContain('Crear incidencia');
  });
});

describe('HotkeysService global shortcuts', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('navega a la creacion de incidencia con la tecla c', async () => {
    const hotkeys = setup();
    const router = TestBed.inject(Router);
    await router.navigateByUrl('/issues');
    hotkeys.registerGlobalShortcuts();
    press('c');
    await vi.waitFor(() => expect(router.url).toBe('/issues/new'));
  });

  it('no dispara los atajos mientras se escribe en un campo', async () => {
    const hotkeys = setup();
    const router = TestBed.inject(Router);
    await router.navigateByUrl('/issues');
    hotkeys.registerGlobalShortcuts();
    const input = document.createElement('input');
    document.body.appendChild(input);
    press('c', {}, input);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(router.url).toBe('/issues');
    input.remove();
  });

  it('no longer binds the slash key, which the command palette host owns', async () => {
    const hotkeys = setup();
    const router = TestBed.inject(Router);
    await router.navigateByUrl('/issues');
    hotkeys.registerGlobalShortcuts();
    const search = document.createElement('input');
    search.setAttribute('aria-label', 'Buscar proyectos e incidencias');
    document.body.appendChild(search);
    press('/');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(document.activeElement).not.toBe(search);
    expect(router.url).toBe('/issues');
    search.remove();
  });
});
