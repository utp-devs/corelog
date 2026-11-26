import styles from './AuthPage.module.css'
import { passwordLevelLabel, scorePassword } from './password-strength'

type PasswordStrengthProps = {
  password: string
}

/** Four-segment strength meter. The text is announced politely; the colors come from tokens via data-level. */
export function PasswordStrength({ password }: PasswordStrengthProps) {
  const level = scorePassword(password)
  const label = passwordLevelLabel(level)

  return (
    <div>
      <div className={styles.meter} data-level={level} aria-hidden="true">
        {[1, 2, 3, 4].map((segment) => (
          <span key={segment} className={styles.segment} data-on={segment <= level ? 'true' : 'false'} />
        ))}
      </div>
      <p aria-live="polite" className={styles.terms}>
        {label ? `Seguridad de la contraseña: ${label}` : ''}
      </p>
    </div>
  )
}
