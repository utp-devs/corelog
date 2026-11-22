import { Select } from 'antd'
import { useEffect, useRef, useState } from 'react'

import { TicketHttpRepository } from '@/adapters/out/http/ticket-http-repository'
import styles from './IssuePicker.module.css'

// Espera antes de buscar: evita una peticion por cada tecla.
const DEBOUNCE_MS = 300

type IssuePickerProps = {
  value: string
  onChange: (issueId: string) => void
  // Incidencia actual: no se ofrece a si misma para vincular.
  excludeId: string
}

// Selector de incidencias con busqueda por titulo. Sustituye a pegar un UUID.
export function IssuePicker({ value, onChange, excludeId }: IssuePickerProps) {
  const [repo] = useState(() => new TicketHttpRepository())
  const [options, setOptions] = useState<{ value: string; label: string }[]>([])
  const [loading, setLoading] = useState(false)
  const timer = useRef<number | undefined>(undefined)

  const search = (query: string) => {
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => {
      setLoading(true)
      repo.list({ page: 1, pageSize: 20, query: query.trim() || undefined })
        .then((page) => {
          setOptions(
            page.items
              .filter((ticket) => ticket.id !== excludeId)
              .map((ticket) => ({ value: ticket.id, label: `${ticket.title} · ${ticket.id.slice(0, 8)}` })),
          )
        })
        .catch(() => setOptions([]))
        .finally(() => setLoading(false))
    }, DEBOUNCE_MS)
  }

  // Carga inicial de sugerencias (las mas recientes) al abrir el panel.
  useEffect(() => {
    search('')
    return () => window.clearTimeout(timer.current)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- la carga inicial solo depende de la incidencia actual
  }, [excludeId])

  return (
    <Select
      showSearch
      allowClear
      value={value || undefined}
      placeholder="Buscar incidencia por título"
      aria-label="Incidencia a vincular"
      filterOption={false}
      onSearch={search}
      onChange={(next: string | undefined) => onChange(next ?? '')}
      options={options}
      loading={loading}
      notFoundContent={loading ? undefined : 'Sin resultados'}
      className={styles.picker}
    />
  )
}
