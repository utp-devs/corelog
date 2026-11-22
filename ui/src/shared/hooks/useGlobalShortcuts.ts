import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'

// Un atajo no se dispara cuando el foco esta en un campo editable.
function isEditable(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

// Atajos: 'c' crea una incidencia y '/' lleva el foco al buscador de la cabecera.
export function useGlobalShortcuts() {
  const navigate = useNavigate()
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.ctrlKey || event.metaKey || event.altKey || isEditable(event.target)) return
      if (event.key === 'c') {
        event.preventDefault()
        navigate('/issues/new')
      } else if (event.key === '/') {
        const search = document.querySelector<HTMLInputElement>('input[aria-label="Buscar proyectos e incidencias"]')
        if (search) {
          event.preventDefault()
          search.focus()
        }
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [navigate])
}
