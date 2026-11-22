import { Avatar } from 'antd'

import { initialsOf } from './initials'
import styles from './AvatarGroup.module.css'

export type AvatarUser = {
  id: string
  name: string
}

type AvatarGroupProps = {
  users: AvatarUser[]
  /** Cuántos avatares mostrar antes de resumir el resto como "+N". */
  max?: number
}

/** Grupo de avatares con iniciales, para responsables y miembros. */
export function AvatarGroup({ users, max = 4 }: AvatarGroupProps) {
  const visible = users.slice(0, max)
  const hidden = users.length - visible.length

  return (
    <Avatar.Group size="small" className={styles.group}>
      {visible.map((user) => (
        <Avatar key={user.id} className={styles.avatar}>
          <span title={user.name}>{initialsOf(user.name)}</span>
        </Avatar>
      ))}
      {hidden > 0 && <Avatar className={styles.avatar}>+{hidden}</Avatar>}
    </Avatar.Group>
  )
}
