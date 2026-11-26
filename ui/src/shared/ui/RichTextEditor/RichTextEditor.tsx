import {
  useEffect,
  useEffectEvent,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type ClipboardEvent,
  type DragEvent,
  type KeyboardEvent,
  type ReactNode,
} from 'react'
import {
  BoldOutlined,
  CheckSquareOutlined,
  CodeOutlined,
  EditOutlined,
  EyeOutlined,
  FieldStringOutlined,
  FileTextOutlined,
  FontSizeOutlined,
  ItalicOutlined,
  LinkOutlined,
  NumberOutlined,
  OrderedListOutlined,
  PictureOutlined,
  StrikethroughOutlined,
  TableOutlined,
  UnorderedListOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Button, Tooltip } from 'antd'

import {
  applyToolbarAction,
  insertAtSelection,
  mentionToken,
  type TextEdit,
  type ToolbarAction,
} from '@/shared/lib/markdown'
import { RichTextView } from '../RichTextView'
import { clearDraft, readDraft, writeDraft } from './draft'
import { detectTrigger, type Trigger, type TriggerKind } from './trigger'
import styles from './RichTextEditor.module.css'
import type { IssueLite, UserLite } from './types'

export type { IssueLite, UserLite } from './types'

export type RichTextEditorProps = {
  value: string
  onChange: (value: string) => void
  /** Called on Ctrl+Enter (or Cmd+Enter). The saved draft is cleared before it is called. */
  onSubmit?: () => void
  placeholder?: string
  minRows?: number
  maxRows?: number
  /** Users for the @ picker. Results are shown as a listbox. */
  searchUsers?: (query: string) => Promise<UserLite[]>
  /** Issues for the # and key picker. */
  searchIssues?: (query: string) => Promise<IssueLite[]>
  /** Uploads an image and returns its public URL. Enables paste, drop and the image button. */
  uploadImage?: (file: File) => Promise<{ url: string }>
  /** Enables draft autosave in sessionStorage under `corelog.draft.<draftKey>`. */
  draftKey?: string
  disabled?: boolean
  ariaLabel: string
}

type PickerKind = TriggerKind

type PopupState = Trigger & { active: number }

type PickerItem = {
  id: string
  label: string
  detail?: string
  insert: string
}

type PickerResults = { key: string; items: PickerItem[] }

const ACCEPTED_IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp']
const SEARCH_DEBOUNCE_MS = 150
const DRAFT_DEBOUNCE_MS = 400

type ToolbarButtonConfig = { action: ToolbarAction; label: string; icon: ReactNode }

const FORMAT_BUTTONS: ToolbarButtonConfig[] = [
  { action: 'bold', label: 'Negrita', icon: <BoldOutlined /> },
  { action: 'italic', label: 'Cursiva', icon: <ItalicOutlined /> },
  { action: 'strikethrough', label: 'Tachado', icon: <StrikethroughOutlined /> },
  { action: 'code', label: 'Código en línea', icon: <CodeOutlined /> },
  { action: 'link', label: 'Enlace', icon: <LinkOutlined /> },
  { action: 'heading', label: 'Encabezado', icon: <FontSizeOutlined /> },
  { action: 'bulletList', label: 'Lista con viñetas', icon: <UnorderedListOutlined /> },
  { action: 'numberedList', label: 'Lista numerada', icon: <OrderedListOutlined /> },
  { action: 'taskList', label: 'Lista de tareas', icon: <CheckSquareOutlined /> },
  { action: 'quote', label: 'Cita', icon: <FieldStringOutlined /> },
  { action: 'codeBlock', label: 'Bloque de código', icon: <FileTextOutlined /> },
  { action: 'table', label: 'Tabla', icon: <TableOutlined /> },
]

export function RichTextEditor({
  value,
  onChange,
  onSubmit,
  placeholder,
  minRows = 4,
  maxRows = 16,
  searchUsers,
  searchIssues,
  uploadImage,
  draftKey,
  disabled = false,
  ariaLabel,
}: RichTextEditorProps) {
  const id = useId()
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pendingSelection = useRef<{ start: number; end: number } | null>(null)
  const dirtyRef = useRef(false)
  const restoredRef = useRef(false)

  const [mode, setMode] = useState<'write' | 'preview'>('write')
  const [popup, setPopup] = useState<PopupState | null>(null)
  const [results, setResults] = useState<PickerResults | null>(null)
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [dragging, setDragging] = useState(false)

  const popupKey = popup ? `${popup.kind}:${popup.query}` : null

  // Debounced search for the open picker. Results are keyed so stale ones are ignored.
  const runSearch = useEffectEvent(async (kind: PickerKind, query: string, key: string) => {
    try {
      let items: PickerItem[] = []
      if (kind === 'user' && searchUsers) {
        const users = await searchUsers(query)
        items = users.map((u) => ({
          id: u.id,
          label: u.name,
          detail: u.email,
          insert: mentionToken({ id: u.id, name: u.name }),
        }))
      } else if (kind === 'issue' && searchIssues) {
        const issues = await searchIssues(query)
        items = issues.map((i) => ({ id: i.key, label: i.key, detail: i.summary, insert: i.key }))
      }
      setResults({ key, items })
    } catch {
      setResults({ key, items: [] })
    }
  })

  useEffect(() => {
    if (!popupKey || !popup) return
    const timer = window.setTimeout(() => {
      void runSearch(popup.kind, popup.query, popupKey)
    }, SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [popupKey, popup])

  const currentItems = popupKey && results?.key === popupKey ? results.items : []
  const pickerLoading = popupKey !== null && results?.key !== popupKey
  const showListbox = popup !== null && (pickerLoading || currentItems.length > 0)

  // Restore the draft once, only when the field starts empty.
  useEffect(() => {
    if (!draftKey || restoredRef.current) return
    restoredRef.current = true
    if (value === '') {
      const draft = readDraft(draftKey)
      if (draft) onChange(draft)
    }
  }, [draftKey, value, onChange])

  // Debounced save after user edits; an empty value clears the draft.
  useEffect(() => {
    if (!draftKey || !dirtyRef.current) return
    const timer = window.setTimeout(() => {
      if (value.trim()) writeDraft(draftKey, value)
      else clearDraft(draftKey)
    }, DRAFT_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [draftKey, value])

  // Autosize between minRows and maxRows.
  useLayoutEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    const computed = window.getComputedStyle(el)
    const lineHeight = parseFloat(computed.lineHeight) || 20
    const padding = (parseFloat(computed.paddingTop) || 0) + (parseFloat(computed.paddingBottom) || 0)
    const min = lineHeight * minRows + padding
    const max = lineHeight * maxRows + padding
    const next = Math.min(Math.max(el.scrollHeight, min), max)
    el.style.height = `${next}px` /* runtime-style */
    el.style.overflowY = el.scrollHeight > max ? 'auto' : 'hidden' /* runtime-style */
  }, [value, minRows, maxRows, mode])

  // Restore the caret after toolbar or picker edits.
  useLayoutEffect(() => {
    const el = textareaRef.current
    const pending = pendingSelection.current
    if (el && pending) {
      pendingSelection.current = null
      el.setSelectionRange(pending.start, pending.end)
    }
  }, [value])

  function emit(next: string) {
    dirtyRef.current = true
    onChange(next)
  }

  function applyEdit(edit: TextEdit) {
    pendingSelection.current = { start: edit.selectionStart, end: edit.selectionEnd }
    emit(edit.value)
    textareaRef.current?.focus()
  }

  function openPickerFrom(text: string, caret: number) {
    const trigger = detectTrigger(text.slice(0, caret))
    setPopup(trigger ? { ...trigger, active: 0 } : null)
  }

  function handleChange(event: ChangeEvent<HTMLTextAreaElement>) {
    const next = event.target.value
    emit(next)
    openPickerFrom(next, event.target.selectionStart)
  }

  function pickItem(item: PickerItem) {
    if (!popup) return
    const insertion = `${item.insert} `
    const next = value.slice(0, popup.start) + insertion + value.slice(popup.end)
    const caret = popup.start + insertion.length
    setPopup(null)
    applyEdit({ value: next, selectionStart: caret, selectionEnd: caret })
  }

  function runToolbar(action: ToolbarAction) {
    const el = textareaRef.current
    if (!el) return
    applyEdit(applyToolbarAction(action, value, el.selectionStart, el.selectionEnd))
  }

  function insertMentionTrigger() {
    const el = textareaRef.current
    if (!el) return
    const edit = insertAtSelection(value, el.selectionStart, el.selectionEnd, '@', '')
    applyEdit(edit)
    openPickerFrom(edit.value, edit.selectionStart)
  }

  function insertIssueTrigger() {
    const el = textareaRef.current
    if (!el) return
    const edit = insertAtSelection(value, el.selectionStart, el.selectionEnd, '#', '')
    applyEdit(edit)
    openPickerFrom(edit.value, edit.selectionStart)
  }

  async function uploadFiles(files: File[]) {
    if (!uploadImage) return
    const images = files.filter((file) => ACCEPTED_IMAGE_TYPES.includes(file.type))
    if (images.length === 0) {
      setUploadError('Formato no admitido. Usa PNG, JPG, GIF o WEBP.')
      return
    }
    const el = textareaRef.current
    const caret = el?.selectionEnd ?? value.length
    setUploadError(null)
    setUploading(true)
    try {
      let inserted = ''
      for (const file of images) {
        const { url } = await uploadImage(file)
        inserted += `![](${url})\n`
      }
      const current = textareaRef.current?.value ?? value
      const next = current.slice(0, caret) + inserted + current.slice(caret)
      const pos = caret + inserted.length
      applyEdit({ value: next, selectionStart: pos, selectionEnd: pos })
    } catch {
      setUploadError('No se pudo subir la imagen. Inténtalo de nuevo.')
    } finally {
      setUploading(false)
    }
  }

  function handlePaste(event: ClipboardEvent<HTMLTextAreaElement>) {
    const files = Array.from(event.clipboardData?.files ?? [])
    if (!uploadImage || files.length === 0) return
    event.preventDefault()
    void uploadFiles(files)
  }

  function handleDragOver(event: DragEvent<HTMLDivElement>) {
    if (!uploadImage || !event.dataTransfer?.types.includes('Files')) return
    event.preventDefault()
    setDragging(true)
  }

  function handleDrop(event: DragEvent<HTMLDivElement>) {
    setDragging(false)
    const files = Array.from(event.dataTransfer?.files ?? [])
    if (!uploadImage || files.length === 0) return
    event.preventDefault()
    void uploadFiles(files)
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (popup) {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        setPopup(null)
        return
      }
      if (currentItems.length > 0) {
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          event.preventDefault()
          const step = event.key === 'ArrowDown' ? 1 : -1
          const count = currentItems.length
          setPopup({ ...popup, active: (popup.active + step + count) % count })
          return
        }
        const plainEnter = event.key === 'Enter' && !event.ctrlKey && !event.metaKey
        if (plainEnter || event.key === 'Tab') {
          event.preventDefault()
          pickItem(currentItems[Math.min(popup.active, currentItems.length - 1)])
          return
        }
      }
    }
    if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
      event.preventDefault()
      if (onSubmit && !disabled) {
        if (draftKey) clearDraft(draftKey)
        dirtyRef.current = false
        onSubmit()
      }
    }
  }

  const listboxId = `${id}-listbox`
  const activeOption = showListbox && popup && currentItems[popup.active] ? `${id}-option-${popup.active}` : undefined
  const pickerLabel = popup?.kind === 'issue' ? 'Incidencias' : 'Personas'

  return (
    <div
      className={styles.root}
      onDragOver={handleDragOver}
      onDragLeave={() => setDragging(false)}
      onDrop={handleDrop}
      data-dragging={dragging ? 'true' : undefined}
    >
      <div className={styles.header}>
        {mode === 'write' && (
          <div className={styles.toolbar} role="toolbar" aria-label="Formato del texto">
            {FORMAT_BUTTONS.map((btn) => (
              <Tooltip key={btn.action} title={btn.label}>
                <Button
                  type="text"
                  size="small"
                  aria-label={btn.label}
                  icon={btn.icon}
                  disabled={disabled}
                  onClick={() => runToolbar(btn.action)}
                />
              </Tooltip>
            ))}
            <Tooltip title="Mencionar persona">
              <Button type="text" size="small" aria-label="Mencionar persona" icon={<UserOutlined />} disabled={disabled} onClick={insertMentionTrigger} />
            </Tooltip>
            <Tooltip title="Enlazar incidencia">
              <Button type="text" size="small" aria-label="Enlazar incidencia" icon={<NumberOutlined />} disabled={disabled} onClick={insertIssueTrigger} />
            </Tooltip>
            {uploadImage && (
              <Tooltip title="Subir imagen">
                <Button
                  type="text"
                  size="small"
                  aria-label="Subir imagen"
                  icon={<PictureOutlined />}
                  disabled={disabled || uploading}
                  onClick={() => fileInputRef.current?.click()}
                />
              </Tooltip>
            )}
            <input
              ref={fileInputRef}
              type="file"
              accept={ACCEPTED_IMAGE_TYPES.join(',')}
              className={styles.visuallyHidden}
              aria-label="Seleccionar imagen"
              tabIndex={-1}
              onChange={(event) => {
                const files = Array.from(event.target.files ?? [])
                event.target.value = ''
                if (files.length > 0) void uploadFiles(files)
              }}
            />
          </div>
        )}
        <div className={styles.modes}>
          <Button
            type="text"
            size="small"
            className={styles.modeButton}
            aria-pressed={mode === 'write'}
            icon={<EditOutlined />}
            onClick={() => setMode('write')}
          >
            Escribir
          </Button>
          <Button
            type="text"
            size="small"
            className={styles.modeButton}
            aria-pressed={mode === 'preview'}
            icon={<EyeOutlined />}
            onClick={() => setMode('preview')}
          >
            Vista previa
          </Button>
        </div>
      </div>

      {mode === 'write' ? (
        <div className={styles.editorArea}>
          <textarea
            ref={textareaRef}
            id={id}
            className={styles.textarea}
            rows={minRows}
            value={value}
            placeholder={placeholder}
            disabled={disabled}
            aria-label={ariaLabel}
            aria-multiline="true"
            aria-keyshortcuts={onSubmit ? 'Control+Enter' : undefined}
            aria-autocomplete={popup ? 'list' : undefined}
            aria-expanded={showListbox}
            aria-controls={showListbox ? listboxId : undefined}
            aria-activedescendant={activeOption}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            onPaste={handlePaste}
            onBlur={() => setPopup(null)}
          />
          {showListbox && popup && (
            <ul id={listboxId} role="listbox" aria-label={pickerLabel} className={styles.listbox}>
              {pickerLoading && <li className={styles.optionDetail}>Buscando…</li>}
              {currentItems.map((item, index) => (
                <li
                  key={item.id}
                  id={`${id}-option-${index}`}
                  role="option"
                  aria-selected={index === popup.active}
                  className={styles.option}
                  onMouseDown={(event) => {
                    event.preventDefault()
                    pickItem(item)
                  }}
                >
                  <span>{item.label}</span>
                  {item.detail && <span className={styles.optionDetail}>{item.detail}</span>}
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : (
        <div className={styles.preview} aria-label="Vista previa" role="region">
          {value.trim() ? <RichTextView value={value} /> : <span className={styles.optionDetail}>Nada que previsualizar.</span>}
        </div>
      )}

      <div className={styles.footer}>
        <span>
          {uploading && <span role="status">Subiendo imagen…</span>}
          {uploadError && <span role="alert" className={styles.error}>{uploadError}</span>}
        </span>
        {onSubmit && <span>Ctrl+Enter para enviar</span>}
      </div>
    </div>
  )
}
