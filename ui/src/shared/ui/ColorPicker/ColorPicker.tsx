import { useId, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { Input } from 'antd'

import { HEX_COLOR_ERROR, PALETTE_SIZE, isHexColor, nextPaletteIndex, paletteIndexOf, paletteValue } from './colorValue'
import styles from './ColorPicker.module.css'

const PALETTE_INDEXES = Array.from({ length: PALETTE_SIZE }, (_, i) => i + 1)

type ColorPickerProps = {
  /** Nombre accesible del grupo, por ejemplo "Color de la etiqueta". */
  label: string
  /** 'palette-N' para la paleta, '#RRGGBB' para un color personalizado, null si no hay. */
  value: string | null
  onChange: (value: string) => void
  /** Muestra el campo hexadecimal para colores personalizados. */
  allowCustom?: boolean
  'data-testid'?: string
}

/** Selector de color de la paleta de tokens, con grupo de radios accesible por teclado. */
export function ColorPicker({ label, value, onChange, allowCustom = true, 'data-testid': testId }: ColorPickerProps) {
  const selected = paletteIndexOf(value)
  const focusable = selected ?? 1
  const swatchRefs = useRef<Array<HTMLButtonElement | null>>([])
  const inputId = useId()
  const errorId = useId()
  const [draft, setDraft] = useState(value && isHexColor(value) ? value : '')

  const invalid = draft !== '' && !isHexColor(draft)
  const customValue = value && isHexColor(value) ? value : null

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next = nextPaletteIndex(event.key, index)
    if (next === null) return
    event.preventDefault()
    onChange(paletteValue(next))
    swatchRefs.current[next - 1]?.focus()
  }

  return (
    <div className={styles.wrapper} data-testid={testId}>
      <div role="radiogroup" aria-label={label} className={styles.swatches}>
        {PALETTE_INDEXES.map((n) => {
          const checked = selected === n
          return (
            <button
              key={n}
              ref={(element) => {
                swatchRefs.current[n - 1] = element
              }}
              type="button"
              role="radio"
              aria-checked={checked}
              aria-label={`Color ${n}`}
              tabIndex={n === focusable ? 0 : -1}
              data-palette={n}
              className={checked ? `${styles.swatch} ${styles.checked}` : styles.swatch}
              onClick={() => onChange(paletteValue(n))}
              onKeyDown={(event) => handleKeyDown(event, n)}
            />
          )
        })}
      </div>

      {customValue && (
        <span
          aria-hidden="true"
          className={styles.preview}
          /* runtime-style */
          style={{ '--cl-custom-color': customValue } as CSSProperties}
        />
      )}

      {allowCustom && (
        <div className={styles.custom}>
          <label htmlFor={inputId} className={styles.customLabel}>
            Color personalizado (hex)
          </label>
          <Input
            id={inputId}
            value={draft}
            placeholder="#RRGGBB"
            maxLength={7}
            status={invalid ? 'error' : undefined}
            aria-invalid={invalid}
            aria-describedby={invalid ? errorId : undefined}
            onChange={(event) => {
              const next = event.target.value.trim()
              setDraft(event.target.value)
              if (isHexColor(next)) onChange(next)
            }}
          />
          {invalid && (
            <span id={errorId} role="alert" className={styles.error}>
              {HEX_COLOR_ERROR}
            </span>
          )}
        </div>
      )}
    </div>
  )
}
