import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SearchInput } from './SearchInput'

describe('SearchInput', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('notifies only the final value after the debounce delay', () => {
    const onChange = vi.fn()
    render(<SearchInput value="" onChange={onChange} debounceMs={300} />)
    const input = screen.getByRole('textbox', { name: 'Buscar' })

    fireEvent.change(input, { target: { value: 'ab' } })
    fireEvent.change(input, { target: { value: 'abc' } })
    expect(onChange).not.toHaveBeenCalled()

    act(() => {
      vi.advanceTimersByTime(300)
    })
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith('abc')
  })

  it('notifies an empty value when cleared', () => {
    const onChange = vi.fn()
    render(<SearchInput value="login" onChange={onChange} />)
    fireEvent.mouseDown(screen.getByRole('textbox', { name: 'Buscar' }))
    const clear = document.querySelector('.ant-input-clear-icon')
    expect(clear).not.toBeNull()
    fireEvent.click(clear as Element)
    expect(onChange).toHaveBeenCalledWith('')
  })
})
