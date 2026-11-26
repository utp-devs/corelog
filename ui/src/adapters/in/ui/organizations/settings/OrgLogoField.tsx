import { Button, Typography } from 'antd'
import { useRef, useState, type ChangeEvent } from 'react'

import type { OrgAdminStore } from '@/adapters/in/state/org-admin.store'
import { validateLogoFile, LOGO_MIME_TYPES } from '@/core/organizations/domain/org-permissions'
import { toast } from '@/shared/ui/Toaster'
import { adminErrorMessage } from '../members/member-messages'
import styles from './OrgSettingsPage.module.css'

interface OrgLogoFieldProps {
  store: OrgAdminStore
  organizationId: string
  organizationName: string
  canEdit: boolean
}

/** Organization logo: preview, upload with client validation (PNG, JPEG, WebP, at most 2 MB) and removal. */
export function OrgLogoField({ store, organizationId, organizationName, canEdit }: OrgLogoFieldProps) {
  const logoUrl = store((s) => s.logoUrl)
  const [busy, setBusy] = useState(false)
  const [clientError, setClientError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const onFileChosen = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    const problem = validateLogoFile(file)
    setClientError(problem)
    if (problem) return
    setBusy(true)
    try {
      await store.getState().uploadLogo(organizationId, file)
      toast.success('Logo actualizado')
    } catch (error) {
      toast.error(adminErrorMessage(error, 'No se pudo subir el logo.'))
    } finally {
      setBusy(false)
    }
  }

  const removeLogo = async () => {
    setBusy(true)
    try {
      await store.getState().removeLogo(organizationId)
      toast.success('Logo eliminado')
    } catch (error) {
      toast.error(adminErrorMessage(error, 'No se pudo quitar el logo.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className={styles.logoField}>
      <div className={styles.logoPreview} aria-hidden={logoUrl ? undefined : true}>
        {logoUrl ? (
          <img src={logoUrl} alt={`Logo de ${organizationName}`} className={styles.logoImage} />
        ) : (
          <Typography.Text type="secondary">Sin logo</Typography.Text>
        )}
      </div>
      <div className={styles.logoActions}>
        <Typography.Text type="secondary">PNG, JPEG o WebP de hasta 2 MB.</Typography.Text>
        <div className={styles.buttonRow}>
          <Button
            onClick={() => inputRef.current?.click()}
            disabled={!canEdit || busy}
            loading={busy}
            title={canEdit ? undefined : 'Solo propietarios y administradores pueden cambiar el logo.'}
          >
            Subir logo
          </Button>
          {logoUrl && (
            <Button danger type="text" onClick={removeLogo} disabled={!canEdit || busy}>
              Quitar logo
            </Button>
          )}
        </div>
        <input
          ref={inputRef}
          type="file"
          accept={LOGO_MIME_TYPES.join(',')}
          className={styles.hiddenInput}
          aria-label="Elegir archivo de logo"
          onChange={onFileChosen}
          disabled={!canEdit}
        />
        {clientError && (
          <Typography.Text type="danger" role="alert">
            {clientError}
          </Typography.Text>
        )}
      </div>
    </div>
  )
}
