import { CopyOutlined } from '@ant-design/icons'
import { Button, Tooltip } from 'antd'

import type { IssueTypeKind } from '@/shared/types/issue'

import { IssueTypeIcon } from '../IssueTypeIcon'
import { toast } from '../Toaster'
import styles from './IssueKey.module.css'

type IssueKeyProps = {
  /** Clave de la incidencia, por ejemplo CL-120. */
  issueKey: string
  href: string
  /** Resumen mostrado en el tooltip. */
  summary?: string
  type?: IssueTypeKind
  /** Resuelta o cerrada: la clave se tacha. */
  resolved?: boolean
  'data-testid'?: string
}

async function copyToClipboard(value: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(value)
    return true
  } catch {
    return false
  }
}

/** Clave de incidencia con icono de tipo, tooltip con el resumen y copia al portapapeles. */
export function IssueKey({ issueKey, href, summary, type, resolved = false, 'data-testid': testId }: IssueKeyProps) {
  async function handleCopy() {
    if (await copyToClipboard(issueKey)) {
      toast.success('Clave copiada')
    }
  }

  return (
    <span className={styles.wrapper} data-resolved={resolved ? 'true' : undefined} data-testid={testId}>
      {type && <IssueTypeIcon type={type} />}
      <Tooltip title={summary}>
        <a href={href} className={resolved ? `${styles.link} ${styles.resolved}` : styles.link}>
          {issueKey}
        </a>
      </Tooltip>
      <span className={styles.copyWrap}>
        <Button type="text" size="small" aria-label="Copiar clave" icon={<CopyOutlined />} onClick={handleCopy} />
      </span>
    </span>
  )
}
