import { Button, Select, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useOrgStore } from '@/adapters/in/state/org.store'
import type { OrgAdminStore } from '@/adapters/in/state/org-admin.store'
import type { Organization, OrganizationMember, OrgRole } from '@/core/organizations/domain/organization'
import { canUseOwnerDangerZone } from '@/core/organizations/domain/org-permissions'
import { ConfirmDangerModal } from '@/shared/ui/ConfirmDangerModal'
import { ConfirmModal } from '@/shared/ui/ConfirmModal'
import { Section } from '@/shared/ui/Section'
import { toast } from '@/shared/ui/Toaster'
import { adminErrorMessage, isLastOwnerError } from '../members/member-messages'
import styles from './OrgSettingsPage.module.css'

interface OrgDangerZoneProps {
  store: OrgAdminStore
  org: Organization
  role: OrgRole | null
  /** Members of the organization; only owners need them (transfer target list). */
  members: OrganizationMember[]
}

/** Danger zone: leave for everyone, transfer ownership and delete for owners only. */
export function OrgDangerZone({ store, org, role, members }: OrgDangerZoneProps) {
  const navigate = useNavigate()
  const currentUserId = useAuthStore((s) => s.user?.id ?? null)
  const isOwner = canUseOwnerDangerZone(role)
  const [leaveOpen, setLeaveOpen] = useState(false)
  const [transferTarget, setTransferTarget] = useState<string | null>(null)
  const [transferOpen, setTransferOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  const transferCandidates = useMemo(
    () => members.filter((member) => member.userId !== currentUserId),
    [members, currentUserId],
  )
  const selectedTarget = transferCandidates.find((member) => member.userId === transferTarget) ?? null

  const leave = async () => {
    setBusy(true)
    try {
      await store.getState().leaveOrganization(org.id)
      setLeaveOpen(false)
      await useOrgStore.getState().fetchOrgs()
      toast.success(`Has salido de ${org.name}`)
      navigate('/')
    } catch (error) {
      setLeaveOpen(false)
      toast.error(
        isLastOwnerError(error)
          ? 'No puedes salir: eres el último propietario. Transfiere la propiedad antes de salir.'
          : adminErrorMessage(error, 'No se pudo salir de la organización.'),
      )
    } finally {
      setBusy(false)
    }
  }

  const transfer = async () => {
    if (!selectedTarget) return
    setBusy(true)
    try {
      await store.getState().transferOwnership(org.id, selectedTarget.userId)
      setTransferOpen(false)
      setTransferTarget(null)
      await useOrgStore.getState().fetchOrgs()
      toast.success(`Propiedad transferida a ${selectedTarget.name}. Ahora eres administrador.`)
    } catch (error) {
      setTransferOpen(false)
      toast.error(adminErrorMessage(error, 'No se pudo transferir la propiedad.'))
    } finally {
      setBusy(false)
    }
  }

  const remove = async () => {
    setBusy(true)
    try {
      await store.getState().deleteOrganization(org.id, org.slug)
      setDeleteOpen(false)
      await useOrgStore.getState().fetchOrgs()
      toast.success('Organización eliminada')
      navigate('/')
    } catch (error) {
      setDeleteOpen(false)
      toast.error(adminErrorMessage(error, 'No se pudo eliminar la organización.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div id="org-danger" className={styles.danger}>
      <Section title="Zona de peligro" description="Estas acciones afectan a toda la organización." ariaLabel="Zona de peligro">
        <div className={styles.dangerRow}>
          <div>
            <Typography.Text strong>Salir de la organización</Typography.Text>
            <Typography.Paragraph type="secondary" className={styles.hint}>
              Perderás el acceso a sus proyectos y equipos. No puedes salir si eres el último propietario.
            </Typography.Paragraph>
          </div>
          <Button danger onClick={() => setLeaveOpen(true)}>
            Salir de la organización
          </Button>
        </div>

        {isOwner ? (
          <>
            <div className={styles.dangerRow}>
              <div>
                <Typography.Text strong>Transferir la propiedad</Typography.Text>
                <Typography.Paragraph type="secondary" className={styles.hint}>
                  El miembro elegido pasa a ser propietario y tú pasas a ser administrador.
                </Typography.Paragraph>
                <Select
                  aria-label="Nuevo propietario"
                  placeholder="Elige un miembro"
                  className={styles.transferSelect}
                  value={transferTarget}
                  onChange={(value: string) => setTransferTarget(value)}
                  options={transferCandidates.map((member) => ({
                    value: member.userId,
                    label: `${member.name} (${member.email})`,
                  }))}
                />
              </div>
              <Button danger disabled={!selectedTarget || busy} onClick={() => setTransferOpen(true)}>
                Transferir propiedad
              </Button>
            </div>
            <div className={styles.dangerRow}>
              <div>
                <Typography.Text strong>Eliminar la organización</Typography.Text>
                <Typography.Paragraph type="secondary" className={styles.hint}>
                  Los proyectos, equipos e incidencias dejan de estar disponibles. El identificador queda reservado.
                </Typography.Paragraph>
              </div>
              <Button danger onClick={() => setDeleteOpen(true)}>
                Eliminar organización
              </Button>
            </div>
          </>
        ) : (
          <Typography.Paragraph type="secondary" className={styles.hint}>
            Solo el propietario puede transferir la propiedad o eliminar la organización.
          </Typography.Paragraph>
        )}
      </Section>

      <ConfirmModal
        open={leaveOpen}
        title="¿Salir de la organización?"
        description={`Perderás el acceso a ${org.name}.`}
        confirmText="Salir"
        danger
        loading={busy}
        onConfirm={leave}
        onCancel={() => setLeaveOpen(false)}
      />

      <ConfirmDangerModal
        open={transferOpen}
        title="Transferir la propiedad"
        description={selectedTarget ? `${selectedTarget.name} será el nuevo propietario y tú pasarás a ser administrador.` : undefined}
        confirmation={selectedTarget?.email ?? ''}
        confirmText="Transferir"
        loading={busy}
        onConfirm={transfer}
        onCancel={() => setTransferOpen(false)}
      />

      <ConfirmDangerModal
        open={deleteOpen}
        title="Eliminar la organización"
        description="Esta acción no se puede deshacer. Escribe el identificador de la organización para confirmar."
        confirmation={org.slug}
        loading={busy}
        onConfirm={remove}
        onCancel={() => setDeleteOpen(false)}
      />
    </div>
  )
}
