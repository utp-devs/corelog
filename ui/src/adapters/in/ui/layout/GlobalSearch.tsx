import { SearchOutlined } from '@ant-design/icons'
import { Button, Input, Popover, Typography } from 'antd'
import { useRef, useState, type ChangeEvent, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { SearchHttpRepository } from '@/adapters/out/http/search-http-repository'
import type { SearchHit } from '@/core/search/domain/search'
import { groupHits } from '@/shared/ui/CommandPalette/groups'
import { openPalette } from '@/shared/ui/CommandPalette/paletteState'
import styles from './GlobalSearch.module.css'

// Espera antes de consultar: evita una peticion por cada tecla.
const DEBOUNCE_MS = 300
const MIN_CHARS = 2

export type GlobalSearchProps = {
  /** Narrow layouts: the field becomes an icon button that opens the command palette. */
  compact?: boolean
}

/**
 * Header search field. Typing shows grouped project and issue hits in a dropdown; Ctrl+K or '/'
 * (or the footer action) opens the full command palette with the same results.
 */
export function GlobalSearch({ compact = false }: GlobalSearchProps) {
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

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') setOpen(false)
  }

  const openFullPalette = () => {
    setOpen(false)
    openPalette(query.trim())
  }

  if (compact) {
    return (
      <Button
        type="text"
        icon={<SearchOutlined />}
        aria-label="Buscar"
        title="Buscar (Ctrl+K)"
        onClick={() => openPalette()}
      />
    )
  }

  const sections = groupHits(hits)

  const panel = (
    <div className={styles.panel}>
      <div className={styles.results} role="region" aria-label="Resultados de búsqueda">
        {message && (
          <Typography.Text type="secondary" className={styles.message} role="status">
            {message}
          </Typography.Text>
        )}
        {sections.map((section) => (
          <div key={section.name} className={styles.section} role="group" aria-label={section.name}>
            <div className={styles.sectionTitle} aria-hidden="true">{section.name}</div>
            <ul className={styles.list}>
              {section.items.map((hit) => (
                <li key={`${hit.kind}-${hit.id}`}>
                  <button type="button" className={styles.result} onClick={() => choose(hit)}>
                    <span className={styles.title}>{hit.title}</span>
                    <span className={styles.subtitle}>{hit.subtitle}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className={styles.footer}>
        <Button type="link" size="small" icon={<SearchOutlined />} onClick={openFullPalette}>
          Abrir paleta de comandos
        </Button>
        <span className={styles.shortcut}>Ctrl+K</span>
      </div>
    </div>
  )

  return (
    <Popover
      open={open && (hits.length > 0 || message !== undefined)}
      onOpenChange={setOpen}
      trigger={[]}
      placement="bottomRight"
      content={panel}
    >
      <Input.Search
        className={styles.field}
        placeholder="Buscar…"
        aria-label="Buscar proyectos e incidencias"
        value={query}
        onChange={onChange}
        onKeyDown={onKeyDown}
        allowClear
      />
    </Popover>
  )
}
