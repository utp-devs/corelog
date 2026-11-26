import { afterEach, describe, expect, it, vi } from 'vitest'

import { clearCommandRegistry, getCommands, registerCommand, subscribeCommands, type Command } from './commandRegistry'

function command(id: string, title = id): Command {
  return { id, title, group: 'Acciones', run: vi.fn() }
}

describe('command registry', () => {
  afterEach(() => {
    clearCommandRegistry()
  })

  it('keeps commands in registration order', () => {
    registerCommand(command('b'))
    registerCommand(command('a'))
    registerCommand(command('c'))
    expect(getCommands().map((item) => item.id)).toEqual(['b', 'a', 'c'])
  })

  it('removes a command through the returned function', () => {
    const remove = registerCommand(command('x'))
    registerCommand(command('y'))
    remove()
    expect(getCommands().map((item) => item.id)).toEqual(['y'])
  })

  it('replaces a command with the same id in place', () => {
    registerCommand(command('a', 'Primero'))
    registerCommand(command('b'))
    registerCommand(command('a', 'Nuevo'))
    expect(getCommands().map((item) => item.title)).toEqual(['Nuevo', 'b'])
  })

  it('does not let a stale cleanup remove a newer registration with the same id', () => {
    const removeOld = registerCommand(command('a', 'Viejo'))
    registerCommand(command('a', 'Nuevo'))
    removeOld()
    expect(getCommands().map((item) => item.title)).toEqual(['Nuevo'])
  })

  it('notifies subscribers on change and stops after unsubscribe', () => {
    const listener = vi.fn()
    const unsubscribe = subscribeCommands(listener)
    const remove = registerCommand(command('a'))
    expect(listener).toHaveBeenCalledTimes(1)
    remove()
    expect(listener).toHaveBeenCalledTimes(2)
    unsubscribe()
    registerCommand(command('b'))
    expect(listener).toHaveBeenCalledTimes(2)
  })
})
