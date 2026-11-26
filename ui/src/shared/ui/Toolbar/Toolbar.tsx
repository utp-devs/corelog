import { MoreOutlined } from '@ant-design/icons'
import { Button, Dropdown } from 'antd'
import type { ReactNode } from 'react'

import { useMediaQuery } from '@/shared/hooks/useMediaQuery'

import styles from './Toolbar.module.css'

export type ToolbarProps = {
  /** Filters and search, aligned to the left. */
  left?: ReactNode
  /** Primary actions, aligned to the right. */
  right?: ReactNode
  /**
   * Secondary actions. Shown inline from 1280 px; below it they move into a "Más" menu.
   */
  secondary?: ReactNode
  /** Sticks under the top bar (offset by --cl-topbar-h) while the page scrolls. */
  sticky?: boolean
}

/** Horizontal action bar: left filters, right actions, wraps on small screens, optional "Más" overflow. */
export function Toolbar({ left, right, secondary, sticky = false }: ToolbarProps) {
  const overflow = useMediaQuery('(max-width: 1279.98px)')
  const hasSecondary = secondary !== undefined && secondary !== null && secondary !== false
  const moveSecondaryToMenu = hasSecondary && overflow

  return (
    <div className={styles.toolbar} data-sticky={sticky ? 'true' : 'false'} role="toolbar" aria-label="Barra de acciones">
      {left !== undefined && <div className={styles.left}>{left}</div>}
      {(right !== undefined || hasSecondary) && (
        <div className={styles.right}>
          {right}
          {hasSecondary && !moveSecondaryToMenu && <div className={styles.inlineSecondary}>{secondary}</div>}
          {moveSecondaryToMenu && (
            <Dropdown
              trigger={['click']}
              placement="bottomRight"
              popupRender={() => <div className={styles.menu}>{secondary}</div>}
            >
              <Button icon={<MoreOutlined aria-hidden="true" />} aria-haspopup="true">
                Más
              </Button>
            </Dropdown>
          )}
        </div>
      )}
    </div>
  )
}
