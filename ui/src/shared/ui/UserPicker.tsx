import { Select, Space } from 'antd'

import { initialsOf } from './initials'
import styles from './UserPicker.module.css'

export type PickableUser = {
  id: string
  name: string
  email?: string
}

type UserPickerProps = {
  users: PickableUser[]
  value?: string
  onChange: (userId?: string) => void
  /** Recibe el texto escrito para que el padre consulte el directorio (con debounce si hace falta). */
  onSearch?: (term: string) => void
  loading?: boolean
  placeholder?: string
  allowClear?: boolean
}

/** Selector de usuario con búsqueda y avatar con iniciales. La carga de datos es responsabilidad del padre. */
export function UserPicker({
  users,
  value,
  onChange,
  onSearch,
  loading = false,
  placeholder = 'Buscar persona',
  allowClear = true,
}: UserPickerProps) {
  return (
    <Select
      showSearch
      filterOption={false}
      allowClear={allowClear}
      value={value}
      placeholder={placeholder}
      loading={loading}
      notFoundContent={loading ? null : 'Sin resultados'}
      onSearch={onSearch}
      onChange={(selected) => onChange(selected ?? undefined)}
      className={styles.picker}
      options={users.map((user) => ({
        value: user.id,
        label: (
          <Space size="small">
            <span className={styles.avatar} aria-hidden="true">{initialsOf(user.name)}</span>
            <span>{user.name}</span>
          </Space>
        ),
        searchLabel: user.name,
      }))}
    />
  )
}
