import { TestBed } from '@angular/core/testing';
import { effect } from '@angular/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CommandRegistry, type Command } from './command-registry';

function command(id: string, title = id): Command {
  return { id, title, group: 'Acciones', run: vi.fn() };
}

describe('command registry', () => {
  let registry: CommandRegistry;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    registry = TestBed.inject(CommandRegistry);
  });

  afterEach(() => {
    registry.clear();
  });

  it('keeps commands in registration order', () => {
    registry.register(command('b'));
    registry.register(command('a'));
    registry.register(command('c'));
    expect(registry.commands().map((item) => item.id)).toEqual(['b', 'a', 'c']);
  });

  it('removes a command through the returned function', () => {
    const remove = registry.register(command('x'));
    registry.register(command('y'));
    remove();
    expect(registry.commands().map((item) => item.id)).toEqual(['y']);
  });

  it('replaces a command with the same id in place', () => {
    registry.register(command('a', 'Primero'));
    registry.register(command('b'));
    registry.register(command('a', 'Nuevo'));
    expect(registry.commands().map((item) => item.title)).toEqual(['Nuevo', 'b']);
  });

  it('does not let a stale cleanup remove a newer registration with the same id', () => {
    const removeOld = registry.register(command('a', 'Viejo'));
    registry.register(command('a', 'Nuevo'));
    removeOld();
    expect(registry.commands().map((item) => item.title)).toEqual(['Nuevo']);
  });

  it('notifies subscribers on change and stops after unsubscribe', () => {
    const listener = vi.fn();
    const ref = TestBed.runInInjectionContext(() =>
      effect(() => {
        listener(registry.commands().length);
      }),
    );
    TestBed.flushEffects();
    expect(listener).toHaveBeenCalledTimes(1);

    const remove = registry.register(command('a'));
    TestBed.flushEffects();
    expect(listener).toHaveBeenCalledTimes(2);

    remove();
    TestBed.flushEffects();
    expect(listener).toHaveBeenCalledTimes(3);

    ref.destroy();
    registry.register(command('b'));
    TestBed.flushEffects();
    expect(listener).toHaveBeenCalledTimes(3);
  });
});
