import { describe, expect, it } from 'vitest'

import { passwordLevelLabel, scorePassword } from './password-strength'

describe('scorePassword', () => {
  it('is 0 for an empty password and never above weak under 8 characters', () => {
    expect(scorePassword('')).toBe(0)
    expect(scorePassword('Abc1!')).toBe(1)
  })

  it('rises with length, mixed case and digits or symbols', () => {
    expect(scorePassword('abcdefgh')).toBe(1)
    expect(scorePassword('abcdefghijkl')).toBe(2)
    expect(scorePassword('Abcdefghijkl')).toBe(3)
    expect(scorePassword('Abcdefghijk1')).toBe(4)
  })

  it('maps levels to Spanish labels', () => {
    expect(passwordLevelLabel(0)).toBe('')
    expect(passwordLevelLabel(4)).toBe('Fuerte')
  })
})
