import { createElement } from 'react'
import { Button, message, notification } from 'antd'

/** Duraciones en segundos (09-design-system.md section 5). */
export const TOAST_DURATION = {
  success: 3,
  info: 3,
  error: 6,
  undo: 6,
} as const

let undoSequence = 0

export type UndoToastOptions = {
  /** Texto de la notificación, por ejemplo "Incidencia eliminada". */
  text: string
  /** Se ejecuta al pulsar Deshacer. */
  onUndo: () => void
}

/** Notificaciones con duración y tono consistentes. Nunca para errores de validación. */
export const toast = {
  success(text: string): void {
    void message.success(text, TOAST_DURATION.success)
  },

  info(text: string): void {
    void message.info(text, TOAST_DURATION.info)
  },

  /** Error con detalle opcional; se mantiene 6 segundos. */
  error(text: string, detail?: string): void {
    notification.error({
      message: text,
      description: detail,
      duration: TOAST_DURATION.error,
      placement: 'topRight',
    })
  },

  /** Confirmación con acción Deshacer durante 6 segundos. */
  undo({ text, onUndo }: UndoToastOptions): void {
    undoSequence += 1
    const key = `undo-${undoSequence}`
    notification.open({
      key,
      message: text,
      duration: TOAST_DURATION.undo,
      placement: 'bottomLeft',
      btn: createElement(
        Button,
        {
          type: 'link',
          size: 'small',
          onClick: () => {
            notification.destroy(key)
            onUndo()
          },
        },
        'Deshacer',
      ),
    })
  },
}
