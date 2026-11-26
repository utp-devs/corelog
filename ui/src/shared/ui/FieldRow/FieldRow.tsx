import { useId, type ReactNode } from 'react'

import styles from './FieldRow.module.css'

/** Props injected into a control when FieldRow children is a function. */
export type FieldControlProps = {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
  'aria-required'?: boolean
}

export type FieldRowProps = {
  label: ReactNode
  /** label above the control (forms, default) or beside it (detail panels). */
  variant?: 'stacked' | 'inline'
  required?: boolean
  /** Helper text under the control. */
  helper?: ReactNode
  /** Validation message; replaces the helper text and marks the control invalid. */
  error?: string
  /**
   * The control. A function receives the ids and aria attributes to put on the real input,
   * so the label, helper and error are associated with it.
   */
  children: ReactNode | ((props: FieldControlProps) => ReactNode)
}

/** Label, control, helper and error for forms and detail panels. Associates the label with the control. */
export function FieldRow({ label, variant = 'stacked', required = false, helper, error, children }: FieldRowProps) {
  const id = useId()
  const helperId = helper !== undefined ? `${id}-helper` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [errorId, helperId].filter(Boolean).join(' ') || undefined

  const control =
    typeof children === 'function'
      ? children({
          id,
          'aria-describedby': describedBy,
          'aria-invalid': error ? true : undefined,
          'aria-required': required ? true : undefined,
        })
      : children

  return (
    <div className={styles.row} data-variant={variant}>
      <div className={styles.labelColumn}>
        <label htmlFor={id} className={styles.label}>
          {label}
          {required && (
            <>
              <span className={styles.requiredMark} aria-hidden="true">
                *
              </span>
              <span className={styles.srOnly}> (obligatorio)</span>
            </>
          )}
        </label>
      </div>
      <div className={styles.controlColumn}>
        {control}
        {error ? (
          <p id={errorId} className={styles.error}>
            {error}
          </p>
        ) : (
          helper !== undefined && (
            <p id={helperId} className={styles.helper}>
              {helper}
            </p>
          )
        )}
      </div>
    </div>
  )
}
