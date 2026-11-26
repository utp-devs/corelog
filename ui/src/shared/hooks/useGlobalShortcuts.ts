import { useNavigate } from 'react-router-dom'

import { useHotkeys } from './useHotkeys'

export type GlobalShortcutOptions = {
  /** Opens the shortcuts dialog ('?'). */
  onHelp?: () => void
  /** Project in context: 'g b' opens its board. */
  projectId?: string | null
}

// Focus the header search field, the same field the old hook targeted.
function focusSearch(event: KeyboardEvent) {
  const search = document.querySelector<HTMLInputElement>('input[aria-label="Buscar proyectos e incidencias"]')
  if (search) {
    event.preventDefault()
    search.focus()
  }
}

/** Global shortcuts of the shell: c, /, ?, g i, g d, g p, g b and n (coming soon). */
export function useGlobalShortcuts(options: GlobalShortcutOptions = {}) {
  const navigate = useNavigate()
  const { onHelp, projectId } = options
  const boardPath = projectId ? `/projects/${encodeURIComponent(projectId)}/board` : '/projects'

  useHotkeys([
    { keys: 'c', group: 'General', description: 'Crear incidencia', run: () => navigate('/issues/new') },
    { keys: '/', group: 'General', description: 'Buscar proyectos e incidencias', run: focusSearch },
    { keys: '?', group: 'General', description: 'Mostrar atajos de teclado', run: () => onHelp?.() },
    { keys: 'n', group: 'General', description: 'Notas rápidas', comingSoon: true },
    { keys: 'g i', group: 'Navegación', description: 'Ir a Incidencias', run: () => navigate('/issues') },
    { keys: 'g d', group: 'Navegación', description: 'Ir a Mi resumen', run: () => navigate('/dashboard') },
    { keys: 'g p', group: 'Navegación', description: 'Ir a Proyectos', run: () => navigate('/projects') },
    {
      keys: 'g b',
      group: 'Navegación',
      description: projectId ? 'Ir al tablero del proyecto' : 'Ir a Proyectos (no hay proyecto abierto)',
      run: () => navigate(boardPath),
    },
  ])
}
