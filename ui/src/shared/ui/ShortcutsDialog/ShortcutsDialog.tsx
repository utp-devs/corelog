import { Modal } from 'antd'
import { Fragment } from 'react'

import { useRegisteredHotkeys, type RegisteredHotkey } from '@/shared/hooks/useHotkeys'
import styles from './ShortcutsDialog.module.css'

export type ShortcutsDialogProps = {
  open: boolean
  onClose: () => void
}

type Group = { name: string; items: RegisteredHotkey[] }

// Groups keep the order of first registration.
function groupHotkeys(hotkeys: readonly RegisteredHotkey[]): Group[] {
  const groups: Group[] = []
  for (const hotkey of hotkeys) {
    const existing = groups.find((group) => group.name === hotkey.group)
    if (existing) {
      existing.items.push(hotkey)
    } else {
      groups.push({ name: hotkey.group, items: [hotkey] })
    }
  }
  return groups
}

/** Lists every shortcut registered with useHotkeys, grouped. Opened with '?'. */
export function ShortcutsDialog({ open, onClose }: ShortcutsDialogProps) {
  const groups = groupHotkeys(useRegisteredHotkeys())

  return (
    <Modal
      open={open}
      title="Atajos de teclado"
      onCancel={onClose}
      footer={null}
      width={520}
      destroyOnHidden
    >
      {groups.length === 0 && <p className={styles.empty}>No hay atajos disponibles en esta pantalla.</p>}
      {groups.map((group) => (
        <section key={group.name} className={styles.group} aria-labelledby={`shortcuts-${group.name}`}>
          <h3 id={`shortcuts-${group.name}`} className={styles.groupTitle}>{group.name}</h3>
          <ul className={styles.list}>
            {group.items.map((item) => (
              <li key={`${item.group}-${item.keys}`} className={styles.item}>
                <span className={styles.description}>{item.description}</span>
                {item.comingSoon ? (
                  <span className={styles.soon}>Próximamente</span>
                ) : (
                  <span className={styles.keys}>
                    {item.keys.split(' ').map((token, index) => (
                      <Fragment key={`${token}-${index}`}>
                        {index > 0 && <span className={styles.then}>luego</span>}
                        <kbd className={styles.kbd}>{token}</kbd>
                      </Fragment>
                    ))}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </section>
      ))}
    </Modal>
  )
}
