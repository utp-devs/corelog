import { KeyOutlined, QuestionCircleOutlined, BookOutlined } from '@ant-design/icons'
import { Button, Dropdown } from 'antd'
import type { MenuProps } from 'antd'

/** Documentation URL comes from the build environment; the entry is hidden when it is not set. */
const DOCS_URL: string | undefined = import.meta.env.VITE_DOCS_URL || undefined

export type HelpMenuProps = {
  onOpenShortcuts: () => void
}

export function HelpMenu({ onOpenShortcuts }: HelpMenuProps) {
  const items: MenuProps['items'] = [
    { key: 'shortcuts', icon: <KeyOutlined />, label: 'Atajos de teclado', onClick: onOpenShortcuts },
    ...(DOCS_URL
      ? [{
          key: 'docs',
          icon: <BookOutlined />,
          label: <a href={DOCS_URL} target="_blank" rel="noopener noreferrer">Documentación</a>,
        }]
      : []),
  ]

  return (
    <Dropdown menu={{ items }} trigger={['click']} placement="bottomRight">
      <Button type="text" icon={<QuestionCircleOutlined />} aria-label="Ayuda" aria-haspopup="menu" />
    </Dropdown>
  )
}
