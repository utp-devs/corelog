import { CheckOutlined, DownOutlined, PlusOutlined, SettingOutlined } from '@ant-design/icons'
import { Button, Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import { useNavigate } from 'react-router-dom'

import { useCurrentOrg, useOrgStore } from '@/adapters/in/state/org.store'
import { canManageOrg } from '@/core/organizations/domain/organization'

import styles from './OrgSwitcher.module.css'

/** Top bar dropdown: the caller's organizations (current one checked), create, and organization settings. */
export function OrgSwitcher() {
  const navigate = useNavigate()
  const orgs = useOrgStore((s) => s.orgs)
  const setCurrentOrg = useOrgStore((s) => s.setCurrentOrg)
  const { org, role } = useCurrentOrg()

  if (!org) return null

  const items: MenuProps['items'] = [
    {
      type: 'group',
      key: 'organizations',
      label: 'Tus organizaciones',
      children: orgs.map((item) => ({
        key: `org-${item.id}`,
        label: item.name,
        icon: item.id === org.id ? <CheckOutlined aria-label="Organización actual" /> : undefined,
        onClick: () => {
          if (item.id !== org.id) {
            setCurrentOrg(item.id)
            navigate('/projects')
          }
        },
      })),
    },
    { type: 'divider', key: 'divider' },
    {
      key: 'create',
      icon: <PlusOutlined />,
      label: 'Crear organización',
      onClick: () => navigate('/onboarding?new=1'),
    },
    ...(canManageOrg(role)
      ? [
          {
            key: 'settings',
            icon: <SettingOutlined />,
            label: 'Configuración de la organización',
            onClick: () => navigate('/organization/settings'),
          },
        ]
      : []),
  ]

  return (
    <Dropdown menu={{ items }} trigger={['click']}>
      <Button type="text" className={styles.trigger} aria-label={`Organización actual: ${org.name}`} aria-haspopup="menu">
        <span className={styles.name}>{org.name}</span>
        <DownOutlined className={styles.caret} aria-hidden="true" />
      </Button>
    </Dropdown>
  )
}
