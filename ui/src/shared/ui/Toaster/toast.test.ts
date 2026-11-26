import type { ReactElement } from 'react'
import { message, notification } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { TOAST_DURATION, toast } from './toast'

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>()
  return {
    ...actual,
    message: { success: vi.fn(), info: vi.fn(), error: vi.fn() },
    notification: { open: vi.fn(), error: vi.fn(), destroy: vi.fn() },
  }
})

describe('toast', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('uses 3 seconds for success and info', () => {
    toast.success('Guardado')
    toast.info('Sincronizando')
    expect(message.success).toHaveBeenCalledWith('Guardado', 3)
    expect(message.info).toHaveBeenCalledWith('Sincronizando', 3)
    expect(TOAST_DURATION.success).toBe(3)
  })

  it('shows errors for 6 seconds with the optional detail', () => {
    toast.error('No se pudo guardar', 'Revisa tu conexión')
    expect(notification.error).toHaveBeenCalledWith(
      expect.objectContaining({ message: 'No se pudo guardar', description: 'Revisa tu conexión', duration: 6 }),
    )
  })

  it('opens an undo notification of 6 seconds with a Deshacer action', () => {
    const onUndo = vi.fn()
    toast.undo({ text: 'Incidencia eliminada', onUndo })
    const args = vi.mocked(notification.open).mock.calls[0][0]
    expect(args.message).toBe('Incidencia eliminada')
    expect(args.duration).toBe(6)
    const button = args.btn as ReactElement<{ children: string; onClick: () => void }>
    expect(button.props.children).toBe('Deshacer')
    button.props.onClick()
    expect(onUndo).toHaveBeenCalledTimes(1)
    expect(notification.destroy).toHaveBeenCalledWith(args.key)
  })

  it('gives each undo notification its own key', () => {
    toast.undo({ text: 'A', onUndo: () => {} })
    toast.undo({ text: 'B', onUndo: () => {} })
    const [first, second] = vi.mocked(notification.open).mock.calls.map((call) => call[0].key)
    expect(first).not.toBe(second)
  })
})
