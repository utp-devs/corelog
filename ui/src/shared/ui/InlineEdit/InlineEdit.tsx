import { useRef, useState, type KeyboardEvent } from 'react'
import { Input, Select, Spin } from 'antd'

import styles from './InlineEdit.module.css'

export type InlineEditType = 'text' | 'select' | 'date' | 'user'
export type InlineEditOption = { value: string; label: string }

export const INLINE_EDIT_SAVE_ERROR = 'No se pudo guardar. Revisa tu conexión e inténtalo de nuevo.'
const EMPTY_TEXT = 'Sin valor'

type InlineEditProps = {
  /** Nombre del campo, por ejemplo "Título". Se usa como nombre accesible. */
  label: string
  type?: InlineEditType
  value: string | null
  /** Opciones para select y user (user: value = id, label = nombre). */
  options?: InlineEditOption[]
  placeholder?: string
  /** Guarda el nuevo valor. Si lanza un error se muestra el aviso y se conserva el valor anterior. */
  onSave: (next: string | null) => void | Promise<void>
  'data-testid'?: string
}

/**
 * Campo editable en línea. Enter guarda, Esc cancela, salir del campo guarda.
 * Durante el guardado muestra un indicador; si falla, vuelve al valor anterior con un aviso.
 */
export function InlineEdit({
  label,
  type = 'text',
  value,
  options = [],
  placeholder = EMPTY_TEXT,
  onSave,
  'data-testid': testId,
}: InlineEditProps) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [pendingValue, setPendingValue] = useState<string | null | undefined>(undefined)
  const [error, setError] = useState<string | null>(null)
  // True only between startEdit and the first commit or cancel, so that a blur
  // that follows Enter or Esc (or the unmount of the editor) does not save again.
  const activeRef = useRef(false)

  const current = value === '' ? null : value
  const pending = pendingValue !== undefined
  const shown = pending ? pendingValue : current
  const shownText = shown === null || shown === undefined ? placeholder : (options.find((o) => o.value === shown)?.label ?? shown)

  function startEdit() {
    activeRef.current = true
    setDraft(current ?? '')
    setError(null)
    setEditing(true)
  }

  function cancelEdit() {
    activeRef.current = false
    setDraft(current ?? '')
    setEditing(false)
  }

  async function save(next: string | null) {
    setPendingValue(next)
    setError(null)
    try {
      await onSave(next)
    } catch {
      setError(INLINE_EDIT_SAVE_ERROR)
    } finally {
      setPendingValue(undefined)
    }
  }

  function commit(raw: string) {
    if (!activeRef.current) return
    activeRef.current = false
    setEditing(false)
    const next = raw === '' ? null : raw
    if (next === current) return
    void save(next)
  }

  function handleEditorKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'Escape') {
      event.preventDefault()
      cancelEdit()
    } else if (event.key === 'Enter' && type !== 'select' && type !== 'user') {
      event.preventDefault()
      commit(draft)
    }
  }

  if (editing) {
    return (
      <div
        className={styles.editor}
        data-testid={testId}
        onKeyDown={handleEditorKeyDown}
        onBlur={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) commit(draft)
        }}
      >
        {type === 'select' || type === 'user' ? (
          <Select<string>
            autoFocus
            aria-label={label}
            className={styles.control}
            value={draft || undefined}
            options={options}
            optionFilterProp="label"
            allowClear
            onChange={(next) => {
              setDraft(next ?? '')
              commit(next ?? '')
            }}
            onClear={() => {
              setDraft('')
              commit('')
            }}
          />
        ) : type === 'date' ? (
          <input
            type="date"
            autoFocus
            aria-label={label}
            className={styles.control}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
          />
        ) : (
          <Input
            autoFocus
            aria-label={label}
            className={styles.control}
            placeholder={placeholder}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
          />
        )}
      </div>
    )
  }

  return (
    <div className={styles.root} data-testid={testId} aria-busy={pending}>
      <button
        type="button"
        className={styles.trigger}
        aria-label={`Editar ${label}: ${shownText}`}
        disabled={pending}
        onClick={startEdit}
      >
        <span
          className={shown === null || shown === undefined ? styles.empty : styles.text}
          title={shown === null || shown === undefined || shownText === '' ? undefined : shownText}
        >
          {shownText}
        </span>
      </button>
      {pending && (
        <span role="status" className={styles.saving}>
          <Spin size="small" />
          <span className={styles.srOnly}>Guardando</span>
        </span>
      )}
      {error && (
        <span role="alert" className={styles.error}>
          {error}
        </span>
      )}
    </div>
  )
}
