import { useBlocker } from 'react-router-dom'

import { ConfirmModal } from '@/shared/ui/ConfirmModal'
import { UNSAVED_DESCRIPTION, UNSAVED_TITLE } from './unsavedChangesCopy'

interface UnsavedChangesBlockerProps {
  dirty: boolean
}

/** Blocks in-app navigation while dirty and asks to leave or keep editing. Only render inside a data router. */
export function UnsavedChangesBlocker({ dirty }: UnsavedChangesBlockerProps) {
  const blocker = useBlocker(({ currentLocation, nextLocation }) => dirty && currentLocation.pathname !== nextLocation.pathname)
  const blocked = blocker.state === 'blocked'

  return (
    <ConfirmModal
      open={blocked}
      title={UNSAVED_TITLE}
      description={UNSAVED_DESCRIPTION}
      confirmText="Salir"
      cancelText="Seguir editando"
      onConfirm={() => {
        if (blocker.state === 'blocked') blocker.proceed()
      }}
      onCancel={() => {
        if (blocker.state === 'blocked') blocker.reset()
      }}
    />
  )
}
