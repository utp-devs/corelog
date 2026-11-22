import { Input, Popover, Typography } from 'antd'
import { useRef, useState, type ChangeEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { SearchHttpRepository } from '@/adapters/out/http/search-http-repository'
import type { SearchHit } from '@/core/search/domain/search'
import styles from './GlobalSearch.module.css'

// Espera antes de consultar: evita una peticion por cada tecla.
const DEBOUNCE_MS = 300
const MIN_CHARS = 2

export function GlobalSearch() {
  const navigate = useNavigate()
  const [repo] = useState(() => new SearchHttpRepository())
  const [query, setQuery] = useState('')
  const [hits, setHits] = useState<SearchHit[]>([])
  const [open, setOpen] = useState(false)
  const [message, setMessage] = useState<string | undefined>(undefined)
  const timer = useRef<number | undefined>(undefined)
  const requestId = useRef(0)

  // Consulta con retardo. Cada peticion lleva un numero; solo la mas reciente puede escribir resultados.
  const onChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value
    setQuery(value)
    window.clearTimeout(timer.current)
    if (value.trim().length < MIN_CHARS) {
      requestId.current += 1
      setHits([])
      setMessage(undefined)
      setOpen(false)
      return
    }
    timer.current = window.setTimeout(() => {
      const current = requestId.current + 1
      requestId.current = current
      repo.search(value.trim())
        .then((found) => {
          if (requestId.current !== current) return
          setHits(found)
          setMessage(found.length === 0 ? 'Sin resultados' : undefined)
          setOpen(true)
        })
        .catch(() => {
          if (requestId.current !== current) return
          setHits([])
          setMessage('No se pudo buscar. Intenta de nuevo.')
          setOpen(true)
        })
    }, DEBOUNCE_MS)
  }

  const choose = (hit: SearchHit) => {
    setOpen(false)
    setQuery('')
    setHits([])
    navigate(hit.link)
  }

  const panel = (
    <ul className={styles.results} aria-label="Resultados de búsqueda">
      {message && <li><Typography.Text type="secondary">{message}</Typography.Text></li>}
      {hits.map((hit) => (
        <li key={`${hit.kind}-${hit.id}`}>
          <button type="button" className={styles.result} onClick={() => choose(hit)}>
            <span className={styles.kind}>{hit.kind === 'project' ? 'Proyecto' : 'Incidencia'}</span>
            <span className={styles.title}>{hit.title}</span>
            <span className={styles.subtitle}>{hit.subtitle}</span>
          </button>
        </li>
      ))}
    </ul>
  )

  return (
    <Popover open={open && (hits.length > 0 || message !== undefined)} onOpenChange={setOpen} trigger={[]} placement="bottomRight" content={panel}>
      <Input.Search
        className={styles.field}
        placeholder="Buscar proyectos e incidencias"
        aria-label="Buscar proyectos e incidencias"
        value={query}
        onChange={onChange}
        allowClear
      />
    </Popover>
  )
}
