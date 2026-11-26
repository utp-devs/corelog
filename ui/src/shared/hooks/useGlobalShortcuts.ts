import { useNavigate } from 'react-router-dom'

import { useHotkeys } from './useHotkeys'

export type GlobalShortcutOptions = {
  /** Opens the shortcuts dialog ('?'). */
  onHelp?: () => void
  /** Project in context: 'g b' opens its board. */
  projectId?: string | null
}

/**
 * Global shortcuts of the shell: c, ?, g i, g d, g p, g b and n (coming soon).
 * The '/' key is not bound here: the command palette host owns it (it opens the palette).
 */
export function useGlobalShortcuts(options: GlobalShortcutOptions = {}) {
  const navigate = useNavigate()
  const { onHelp, projectId } = options
  const boardPath = projectId ? `/projects/${encodeURIComponent(projectId)}/board` : '/projects'

  useHotkeys([
    { keys: 'c', group: 'General', description: 'Crear incidencia', run: () => navigate('/issues/new') },
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
