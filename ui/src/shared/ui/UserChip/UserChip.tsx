import { Avatar } from '../Avatar'
import styles from './UserChip.module.css'

export type UserChipUser = {
  id: string
  name: string
  /** Rol o cargo mostrado debajo del nombre. */
  role?: string | null
  avatarUrl?: string | null
}

type UserChipProps = {
  user: UserChipUser
  /** Si existe, el chip es un enlace al perfil. */
  href?: string
  size?: 24 | 32
  'data-testid'?: string
}

/** Avatar con nombre y, opcionalmente, rol y enlace. */
export function UserChip({ user, href, size = 24, 'data-testid': testId }: UserChipProps) {
  const content = (
    <>
      <Avatar id={user.id} name={user.name} src={user.avatarUrl} size={size} />
      <span className={styles.text}>
        <span className={styles.name} title={user.name}>
          {user.name}
        </span>
        {user.role && (
          <span className={styles.role} title={user.role}>
            {user.role}
          </span>
        )}
      </span>
    </>
  )

  if (href) {
    return (
      <a href={href} className={styles.chip} data-testid={testId}>
        {content}
      </a>
    )
  }

  return (
    <span className={styles.chip} data-testid={testId}>
      {content}
    </span>
  )
}
