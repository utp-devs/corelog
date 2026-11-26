import { useContext, useEffect, type ReactNode } from 'react'
import { UNSAFE_DataRouterContext } from 'react-router-dom'

import { UnsavedChangesBlocker } from './UnsavedChangesBlocker'

/**
 * Guards a form against losing unsaved changes.
 * - Browser reload or close: the native beforeunload prompt while `dirty` is true.
 * - In-app navigation (data router): the navigation is blocked and a dialog asks to leave or keep editing.
 *   Outside a data router (plain MemoryRouter, tests) only the beforeunload part applies.
 *
 * Render the returned node somewhere inside the component: it is the dialog (or nothing).
 */
export function useUnsavedChangesGuard(dirty: boolean): ReactNode {
  useBeforeUnloadWarning(dirty)
  const inDataRouter = useContext(UNSAFE_DataRouterContext) !== null
  return inDataRouter ? <UnsavedChangesBlocker dirty={dirty} /> : null
}

function useBeforeUnloadWarning(dirty: boolean) {
  useEffect(() => {
    if (!dirty) return
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
}
