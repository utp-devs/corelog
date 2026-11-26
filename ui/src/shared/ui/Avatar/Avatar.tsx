import { initialsOf } from '../initials'
import { avatarColorIndex } from './avatarColor'
import styles from './Avatar.module.css'

export type AvatarSize = 16 | 20 | 24 | 32 | 40 | 64

type AvatarProps = {
  /** Identificador estable del usuario: decide el color de fondo. */
  id: string
  /** Nombre visible: origen de las iniciales y nombre accesible. */
  name: string
  /** Foto de perfil; si falta se muestran las iniciales. */
  src?: string | null
  size?: AvatarSize
  'data-testid'?: string
}

/** Avatar con foto o iniciales. El color de fondo es fijo para cada id. */
export function Avatar({ id, name, src, size = 32, 'data-testid': testId }: AvatarProps) {
  const sizeClass = styles[`size${size}`]

  if (src) {
    return <img src={src} alt={name} className={`${styles.avatar} ${sizeClass}`} data-testid={testId} />
  }

  const toneClass = styles[`tone${avatarColorIndex(id)}`]
  return (
    <span role="img" aria-label={name} className={`${styles.avatar} ${sizeClass} ${toneClass}`} data-testid={testId}>
      <span aria-hidden="true">{initialsOf(name)}</span>
    </span>
  )
}
