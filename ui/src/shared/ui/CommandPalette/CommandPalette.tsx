import {
  ArrowRightOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  ProjectOutlined,
  ReloadOutlined,
  SearchOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import { Button } from 'antd'
import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

import type { SearchHit } from '@/core/search/domain/search'
import { useCommands, type Command, type CommandGroup } from './commandRegistry'
import { groupHits, matchesQuery } from './groups'
import { getRecent } from './recent'
import styles from './CommandPalette.module.css'

/** Wait after the last keystroke before asking the server. */
export const PALETTE_DEBOUNCE_MS = 150
/** Same minimum as the /search endpoint (minLength 2). */
export const PALETTE_MIN_CHARS = 2

export type SearchResult = SearchHit

export type CommandPaletteProps = {
  open: boolean
  /** Text to prefill the input with when the palette opens. */
  seed?: string
  onClose: () => void
  /** Remote search for projects and issues. A rejection shows the error state. */
  search: (query: string) => Promise<SearchResult[]>
  /** Called with a search hit or a recent item the user picked. The palette closes first. */
  onOpenHit: (hit: SearchResult) => void
}

type PaletteSection = CommandGroup | 'Recientes' | 'Proyectos' | 'Incidencias'

type Option = {
  key: string
  section: PaletteSection
  title: string
  subtitle?: string
  onSelect: () => void
}

type RemoteState = {
  /** Trimmed query the result belongs to. */
  term: string
  status: 'ready' | 'error'
  hits: SearchResult[]
}

const SECTION_ICON: Record<PaletteSection, ReactNode> = {
  Acciones: <ThunderboltOutlined />,
  Navegación: <ArrowRightOutlined />,
  Recientes: <ClockCircleOutlined />,
  Proyectos: <ProjectOutlined />,
  Incidencias: <FileTextOutlined />,
}

const SECTION_ORDER: PaletteSection[] = ['Acciones', 'Navegación', 'Recientes', 'Proyectos', 'Incidencias']

/**
 * Modal command palette. It is mounted only while open, so the input starts from `seed` and the
 * focus is moved in on mount and restored to the previous element on unmount.
 */
export function CommandPalette({ open, seed = '', onClose, search, onOpenHit }: CommandPaletteProps) {
  if (!open) return null
  return <PaletteDialog seed={seed} onClose={onClose} search={search} onOpenHit={onOpenHit} />
}

type PaletteDialogProps = Omit<CommandPaletteProps, 'open'>

function PaletteDialog({ seed = '', onClose, search, onOpenHit }: PaletteDialogProps) {
  const [query, setQuery] = useState(seed)
  const [active, setActive] = useState(0)
  const [remote, setRemote] = useState<RemoteState>({ term: '', status: 'ready', hits: [] })
  const [retryToken, setRetryToken] = useState(0)
  const [retrying, setRetrying] = useState(false)
  const dialogRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listId = useId()
  const commands = useCommands()
  const searchRef = useRef(search)

  useEffect(() => {
    searchRef.current = search
  })

  // Focus moves into the palette on mount and goes back to the previous element on unmount.
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null
    inputRef.current?.focus()
    return () => {
      previous?.focus()
    }
  }, [])

  // Keeps focus inside the dialog while it is open, including programmatic focus from elsewhere.
  useEffect(() => {
    const onFocusIn = (event: FocusEvent) => {
      const dialog = dialogRef.current
      if (dialog && event.target instanceof Node && !dialog.contains(event.target)) {
        inputRef.current?.focus()
      }
    }
    document.addEventListener('focusin', onFocusIn)
    return () => document.removeEventListener('focusin', onFocusIn)
  }, [])

  const term = query.trim()
  const searchable = term.length >= PALETTE_MIN_CHARS

  // Debounced remote search. Cancelling the timer and the pending promise on every change means
  // only the latest query can write its result.
  useEffect(() => {
    if (!searchable) return
    let cancelled = false
    const timer = window.setTimeout(() => {
      searchRef.current(term)
        .then((hits) => {
          if (!cancelled) setRemote({ term, status: 'ready', hits })
        })
        .catch(() => {
          if (!cancelled) setRemote({ term, status: 'error', hits: [] })
        })
        .finally(() => {
          if (!cancelled) setRetrying(false)
        })
    }, PALETTE_DEBOUNCE_MS)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [term, searchable, retryToken])

  const stale = searchable && remote.term !== term
  const loading = searchable && (stale || retrying)
  const hasError = searchable && !stale && !retrying && remote.status === 'error'
  const remoteHits = searchable && !stale && remote.status === 'ready' ? remote.hits : []

  // Built on every render: the recent list is small and localStorage reads are synchronous.
  const options: Option[] = []
  for (const command of commands) {
    if (matchesQuery([command.title, ...(command.keywords ?? [])].join(' '), term)) {
      options.push(commandOption(command, onClose))
    }
  }
  for (const item of getRecent()) {
    if (matchesQuery(`${item.title} ${item.subtitle}`, term)) {
      options.push(hitOption('Recientes', item, onClose, onOpenHit))
    }
  }
  for (const section of groupHits(remoteHits)) {
    for (const hit of section.items) options.push(hitOption(section.name, hit, onClose, onOpenHit))
  }
  // Index N must match the Nth option in the DOM, so options are ordered by section here.
  options.sort((a, b) => SECTION_ORDER.indexOf(a.section) - SECTION_ORDER.indexOf(b.section))

  const safeActive = Math.min(active, Math.max(options.length - 1, 0))
  const activeOption = options[safeActive]
  const sections = SECTION_ORDER.map((name) => ({
    name,
    items: options.map((option, index) => ({ option, index })).filter(({ option }) => option.section === name),
  })).filter((section) => section.items.length > 0)

  const onChange = (value: string) => {
    setQuery(value)
    setActive(0)
  }

  const retry = () => {
    setRetrying(true)
    setRetryToken((value) => value + 1)
  }

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
      return
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      if (options.length === 0) return
      const step = event.key === 'ArrowDown' ? 1 : -1
      setActive((safeActive + step + options.length) % options.length)
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      activeOption?.onSelect()
      return
    }
    if (event.key === 'Tab') {
      // The input and the retry button are the only focusable elements, so Tab cycles between them.
      const focusable = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>('input, button') ?? [])
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
  }

  return createPortal(
    <div
      className={styles.overlay}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Paleta de comandos"
        className={styles.dialog}
        onKeyDown={onKeyDown}
      >
        <div className={styles.inputRow}>
          <SearchOutlined className={styles.inputIcon} aria-hidden="true" />
          <input
            ref={inputRef}
            type="text"
            role="combobox"
            aria-label="Buscar o escribir un comando"
            aria-expanded={options.length > 0}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={activeOption ? optionId(listId, safeActive) : undefined}
            placeholder="Buscar proyectos, incidencias o comandos…"
            className={styles.input}
            value={query}
            onChange={(event) => onChange(event.target.value)}
            autoComplete="off"
          />
        </div>

        <div role="status" aria-live="polite" className={styles.status}>
          {loading && <span>Buscando…</span>}
          {hasError && (
            <span className={styles.error}>
              No se pudo buscar. Intenta de nuevo.{' '}
              <Button type="link" size="small" icon={<ReloadOutlined />} onClick={retry}>
                Reintentar
              </Button>
            </span>
          )}
          {!searchable && <span>Escribe al menos 2 caracteres para buscar proyectos e incidencias.</span>}
          {searchable && !loading && !hasError && options.length === 0 && <span>Sin resultados</span>}
        </div>

        {options.length > 0 && (
          <div id={listId} role="listbox" aria-label="Resultados" className={styles.list}>
            {sections.map((section) => (
              <div key={section.name} role="group" aria-label={section.name} className={styles.group}>
                <div className={styles.groupTitle} aria-hidden="true">
                  {section.name}
                </div>
                {section.items.map(({ option, index }) => (
                  <div
                    key={option.key}
                    id={optionId(listId, index)}
                    role="option"
                    aria-selected={index === safeActive}
                    className={index === safeActive ? `${styles.option} ${styles.optionActive}` : styles.option}
                    onMouseEnter={() => setActive(index)}
                    onClick={() => option.onSelect()}
                  >
                    <span className={styles.optionIcon} aria-hidden="true">
                      {SECTION_ICON[option.section]}
                    </span>
                    <span className={styles.optionTitle}>{option.title}</span>
                    {option.subtitle && <span className={styles.optionSubtitle}>{option.subtitle}</span>}
                  </div>
                ))}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}

function optionId(listId: string, index: number): string {
  return `${listId}-option-${index}`
}

function commandOption(command: Command, close: () => void): Option {
  return {
    key: `command-${command.id}`,
    section: command.group,
    title: command.title,
    onSelect: () => {
      close()
      command.run()
    },
  }
}

function hitOption(
  section: PaletteSection,
  hit: SearchResult,
  close: () => void,
  onOpenHit: (hit: SearchResult) => void,
): Option {
  return {
    key: `${section}-${hit.kind}-${hit.id}`,
    section,
    title: hit.title,
    subtitle: hit.subtitle,
    onSelect: () => {
      close()
      onOpenHit(hit)
    },
  }
}
