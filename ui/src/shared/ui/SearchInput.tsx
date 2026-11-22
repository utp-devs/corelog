import { SearchOutlined } from '@ant-design/icons'
import { Input } from 'antd'
import { useEffect, useState } from 'react'

type SearchInputProps = {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  /** Tiempo de espera tras la última tecla antes de notificar el cambio. */
  debounceMs?: number
  ariaLabel?: string
}

/** Búsqueda con icono, botón de limpiar y debounce. Notifica el valor final, no cada tecla. */
export function SearchInput({ value, onChange, placeholder = 'Buscar', debounceMs = 300, ariaLabel = 'Buscar' }: SearchInputProps) {
  const [draft, setDraft] = useState(value)
  const [lastValue, setLastValue] = useState(value)

  // Sincroniza el borrador cuando el padre cambia el valor (por ejemplo, al limpiar filtros).
  // Se ajusta durante el render, no en un efecto, para evitar renderizados en cascada.
  if (value !== lastValue) {
    setLastValue(value)
    setDraft(value)
  }

  useEffect(() => {
    if (draft === value) return
    const timer = window.setTimeout(() => onChange(draft), debounceMs)
    return () => window.clearTimeout(timer)
  }, [draft, value, onChange, debounceMs])

  return (
    <Input
      allowClear
      prefix={<SearchOutlined aria-hidden="true" />}
      placeholder={placeholder}
      aria-label={ariaLabel}
      value={draft}
      onChange={(event) => setDraft(event.target.value)}
      onClear={() => onChange('')}
    />
  )
}
