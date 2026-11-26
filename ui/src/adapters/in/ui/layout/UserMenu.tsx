import {
  ColumnHeightOutlined,
  LogoutOutlined,
  MoonOutlined,
  SunOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import { useNavigate } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useThemeMode } from '@/app/theme-context'
import type { Role } from '@/core/auth/domain/user'
import { Avatar } from '@/shared/ui/Avatar'

import { useDensity } from './preferences'
import styles from './UserMenu.module.css'

const ROLE_LABELS: Record<Role, string> = {
  admin: 'Administrador',
  user: 'Usuario',
}

/** Avatar menu: profile, theme, density and sign out. */
export function UserMenu() {
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const { themeMode, toggleTheme } = useThemeMode()
  const [density, setDensity] = useDensity()

  if (!user) return null

  const items: MenuProps['items'] = [
    {
      key: 'identity',
      disabled: true,
      label: (
        <span className={styles.identity}>
          <span className={styles.identityName}>{user.name}</span>
          <span className={styles.identityMeta}>{user.email} · {ROLE_LABELS[user.role]}</span>
        </span>
      ),
    },
    { type: 'divider', key: 'divider-identity' },
    { key: 'profile', icon: <UserOutlined />, label: 'Mi perfil', onClick: () => navigate('/profile') },
    {
      key: 'theme',
      icon: themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />,
      label: themeMode === 'dark' ? 'Usar tema claro' : 'Usar tema oscuro',
      onClick: toggleTheme,
    },
    {
      key: 'density',
      icon: <ColumnHeightOutlined />,
      label: density === 'compact' ? 'Densidad cómoda' : 'Densidad compacta',
      onClick: () => setDensity(density === 'compact' ? 'comfortable' : 'compact'),
    },
    { type: 'divider', key: 'divider-session' },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      danger: true,
      label: 'Cerrar sesión',
      onClick: () => {
        logout()
        navigate('/login')
      },
    },
  ]

  return (
    <Dropdown menu={{ items }} trigger={['click']} placement="bottomRight">
      <button type="button" className={styles.trigger} aria-label={`Menú de ${user.name}`} aria-haspopup="menu">
        <Avatar id={user.id} name={user.name} size={32} />
      </button>
    </Dropdown>
  )
}
