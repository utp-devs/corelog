import { Link } from 'react-router-dom'

import styles from './Breadcrumbs.module.css'

export type BreadcrumbItem = {
  label: string
  /** Route to link to. The last item is the current page and is never a link. */
  to?: string
}

export type BreadcrumbsProps = {
  items: BreadcrumbItem[]
}

/**
 * Route trail. Middle items are hidden below 768 px (CSS) so the first and last stay visible.
 * The last item is marked aria-current="page".
 */
export function Breadcrumbs({ items }: BreadcrumbsProps) {
  if (items.length === 0) return null
  const lastIndex = items.length - 1

  return (
    <nav aria-label="Ruta de navegación" className={styles.nav}>
      <ol className={styles.list}>
        {items.map((item, index) => {
          const isCurrent = index === lastIndex
          const isMiddle = index > 0 && !isCurrent
          const itemClass = isMiddle ? `${styles.item} ${styles.middle}` : styles.item
          return (
            <li key={`${index}-${item.label}`} className={itemClass}>
              {isCurrent ? (
                <span aria-current="page" className={styles.current}>
                  {item.label}
                </span>
              ) : item.to !== undefined ? (
                <Link to={item.to} className={styles.link}>
                  {item.label}
                </Link>
              ) : (
                <span className={styles.plain}>{item.label}</span>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
