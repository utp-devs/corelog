import { useCallback, useContext, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { ThemeContext } from '@/app/theme-context'
import { SearchHttpRepository } from '@/adapters/out/http/search-http-repository'
import { useHotkeys } from '@/shared/hooks/useHotkeys'
import { CommandPalette, type SearchResult } from '@/shared/ui/CommandPalette/CommandPalette'
import { registerCommand } from '@/shared/ui/CommandPalette/commandRegistry'
import { closePalette, openPalette, togglePalette, usePaletteState } from '@/shared/ui/CommandPalette/paletteState'

/** Opens the palette on '/' (registered in the shortcuts dialog) and toggles it on Ctrl+K / Cmd+K. */
function usePaletteHotkeys(): void {
  useHotkeys([
    { keys: '/', group: 'General', description: 'Abrir la paleta de comandos', run: () => openPalette() },
  ])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        togglePalette()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])
}

/**
 * Mounts the command palette and wires it to the real search endpoint and the router.
 * Mount once in the authenticated shell (inside the router and the theme provider).
 */
export function CommandPaletteHost() {
  const navigate = useNavigate()
  const theme = useContext(ThemeContext)
  const [repo] = useState(() => new SearchHttpRepository())
  const { open, seed } = usePaletteState()
  const toggleTheme = theme?.toggleTheme
  const toggleRef = useRef(toggleTheme)

  useEffect(() => {
    toggleRef.current = toggleTheme
  })

  usePaletteHotkeys()

  const search = useCallback((query: string) => repo.search(query), [repo])

  const onOpenHit = useCallback(
    (hit: SearchResult) => {
      navigate(hit.link)
    },
    [navigate],
  )

  // Built-in commands. Each one is removed again when the host unmounts.
  useEffect(() => {
    const unregister = [
      registerCommand({ id: 'nav-dashboard', title: 'Ir a Mi resumen', keywords: ['dashboard', 'inicio'], group: 'Navegación', run: () => navigate('/dashboard') }),
      registerCommand({ id: 'nav-issues', title: 'Ir a Incidencias', keywords: ['issues', 'tickets'], group: 'Navegación', run: () => navigate('/issues') }),
      registerCommand({ id: 'nav-projects', title: 'Ir a Proyectos', keywords: ['projects'], group: 'Navegación', run: () => navigate('/projects') }),
      registerCommand({ id: 'nav-teams', title: 'Ir a Equipos', keywords: ['teams'], group: 'Navegación', run: () => navigate('/teams') }),
      registerCommand({ id: 'nav-profile', title: 'Ir a Perfil', keywords: ['profile', 'cuenta'], group: 'Navegación', run: () => navigate('/profile') }),
      registerCommand({ id: 'act-create-issue', title: 'Crear incidencia', keywords: ['nueva', 'create', 'issue'], group: 'Acciones', run: () => navigate('/issues/new') }),
      registerCommand({ id: 'act-theme', title: 'Cambiar tema', keywords: ['claro', 'oscuro', 'theme'], group: 'Acciones', run: () => toggleRef.current?.() }),
      registerCommand({
        id: 'act-shortcuts',
        title: 'Atajos de teclado',
        keywords: ['teclas', 'shortcuts', 'ayuda'],
        group: 'Acciones',
        // The shortcuts dialog listens for '?' on window (useGlobalShortcuts), so the command re-sends that key.
        run: () => window.dispatchEvent(new KeyboardEvent('keydown', { key: '?', bubbles: true })),
      }),
    ]
    return () => unregister.forEach((remove) => remove())
  }, [navigate])

  return (
    <CommandPalette
      open={open}
      seed={seed}
      onClose={closePalette}
      search={search}
      onOpenHit={onOpenHit}
    />
  )
}
