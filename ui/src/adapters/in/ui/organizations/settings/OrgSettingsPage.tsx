import { zodResolver } from '@hookform/resolvers/zod'
import { Alert, Button, Input, InputNumber, Select, Typography } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'

import { useCurrentOrg, useOrgStore } from '@/adapters/in/state/org.store'
import { useOrgAdminStore, type OrgAdminStore } from '@/adapters/in/state/org-admin.store'
import { timezoneOptions } from '@/core/people/domain/timezones'
import { canEditOrgSettings, canUseOwnerDangerZone } from '@/core/organizations/domain/org-permissions'
import type { Organization } from '@/core/organizations/domain/organization'
import { EmptyState } from '@/shared/ui/EmptyState'
import { FieldRow } from '@/shared/ui/FieldRow/FieldRow'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { Section } from '@/shared/ui/Section/Section'
import { toast } from '@/shared/ui/Toaster'
import { adminErrorMessage } from '../members/member-messages'
import { OrgDangerZone } from './OrgDangerZone'
import { OrgLogoField } from './OrgLogoField'
import {
  settingsFieldErrors,
  settingsPatchOf,
  settingsSchema,
  settingsValuesOf,
  type SettingsFormField,
  type SettingsFormValues,
} from './settings-form'
import styles from './OrgSettingsPage.module.css'

export interface OrgSettingsPageProps {
  /** Injected for tests; the app uses the application singleton. */
  store?: OrgAdminStore
}

const WHO_CAN_OPTIONS = [
  { value: 'members', label: 'Todos los miembros' },
  { value: 'admins', label: 'Solo administradores' },
]

const VISIBILITY_OPTIONS = [
  { value: 'open', label: 'Abierta: visible para todos los miembros' },
  { value: 'private', label: 'Privada: solo para quienes se añaden' },
]

const SECTION_LINKS = [
  { href: '#org-general', label: 'General' },
  { href: '#org-policies', label: 'Políticas' },
  { href: '#org-time', label: 'Tiempo' },
  { href: '#org-danger', label: 'Zona de peligro' },
]

/** Settings of the active organization (S-20). Owners and admins edit; other roles read. */
export function OrgSettingsPage({ store = useOrgAdminStore }: OrgSettingsPageProps) {
  const { org, role } = useCurrentOrg()
  const canEdit = canEditOrgSettings(role)
  const [saving, setSaving] = useState(false)
  const members = store((s) => s.members)
  const organizationId = org?.id ?? null
  const ownerView = canUseOwnerDangerZone(role)

  useEffect(() => {
    if (!organizationId) return
    void store.getState().loadLogo(organizationId)
  }, [store, organizationId])

  useEffect(() => {
    if (!organizationId || !ownerView) return
    void store.getState().fetchMembers(organizationId)
  }, [store, organizationId, ownerView])

  if (!org) {
    return (
      <PageContainer variant="reading" pageName="org-settings" ready>
        <EmptyState title="Elige una organización" description="La configuración corresponde a la organización activa." />
      </PageContainer>
    )
  }

  return (
    <PageContainer
      variant="reading"
      pageName="org-settings"
      ready
      sidebar={
        <nav aria-label="Secciones de configuración" className={styles.sectionNav}>
          {SECTION_LINKS.map((link) => (
            <a key={link.href} href={link.href} className={styles.sectionLink}>
              {link.label}
            </a>
          ))}
        </nav>
      }
    >
      <PageHeader title="Configuración de la organización" description={org.name} />

      {!canEdit && (
        <Alert
          type="info"
          showIcon
          className={styles.banner}
          message="Solo lectura"
          description="Solo los propietarios y administradores pueden cambiar la configuración. Puedes ver los valores actuales."
        />
      )}

      <SettingsForm
        key={`${org.id}|${org.updatedAt}`}
        org={org}
        canEdit={canEdit}
        saving={saving}
        setSaving={setSaving}
        store={store}
      />

      <OrgDangerZone store={store} org={org} role={role} members={members} />
    </PageContainer>
  )
}

interface SettingsFormProps {
  org: Organization
  canEdit: boolean
  saving: boolean
  setSaving: (value: boolean) => void
  store: OrgAdminStore
}

function SettingsForm({ org, canEdit, saving, setSaving, store }: SettingsFormProps) {
  const values = useMemo(() => settingsValuesOf(org), [org])
  const timezones = useMemo(() => timezoneOptions(org.settings.timezone), [org.settings.timezone])
  const {
    control,
    handleSubmit,
    setError,
    reset,
    formState: { errors, isDirty },
  } = useForm<SettingsFormValues>({
    resolver: zodResolver(settingsSchema),
    values,
    disabled: !canEdit,
  })

  // Dirty guard: warn before leaving the page with unsaved changes.
  useEffect(() => {
    if (!isDirty) return
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault()
    }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [isDirty])

  const save = handleSubmit(async (formValues) => {
    setSaving(true)
    try {
      const updated = await store.getState().updateOrganization(org.id, settingsPatchOf(formValues))
      useOrgStore.setState((state) => ({
        orgs: state.orgs.map((item) => (item.id === updated.id ? { ...updated, role: item.role } : item)),
      }))
      toast.success('Configuración guardada')
    } catch (error) {
      const fieldMessages = settingsFieldErrors(error)
      const names = Object.keys(fieldMessages) as SettingsFormField[]
      if (names.length === 0) {
        toast.error(adminErrorMessage(error, 'No se pudo guardar la configuración.'))
      }
      for (const name of names) {
        setError(name, { type: 'server', message: fieldMessages[name] })
      }
    } finally {
      setSaving(false)
    }
  })

  const message = (field: SettingsFormField) => errors[field]?.message
  const dirtyAndEditable = isDirty && canEdit

  return (
    <form aria-label="Configuración de la organización" onSubmit={save} noValidate className={styles.form}>
      <div id="org-general">
        <Section title="General" ariaLabel="General">
          <FieldRow label="Nombre" required error={message('name')}>
            {(props) => (
              <Controller
                name="name"
                control={control}
                render={({ field }) => <Input {...field} {...props} maxLength={120} disabled={!canEdit} />}
              />
            )}
          </FieldRow>
          <FieldRow label="Identificador" helper="Se usa en las URL y no puede cambiarse.">
            <Typography.Text code>{org.slug}</Typography.Text>
          </FieldRow>
          <FieldRow label="Descripción" error={message('description')}>
            {(props) => (
              <Controller
                name="description"
                control={control}
                render={({ field }) => (
                  <Input.TextArea {...field} {...props} rows={3} maxLength={500} disabled={!canEdit} />
                )}
              />
            )}
          </FieldRow>
          <FieldRow label="Logo">
            {() => (
              <OrgLogoField
                store={store}
                organizationId={org.id}
                organizationName={org.name}
                canEdit={canEdit}
              />
            )}
          </FieldRow>
        </Section>
      </div>

      <div id="org-policies">
        <Section title="Políticas" description="Quién puede crear proyectos y equipos, y la visibilidad por defecto." ariaLabel="Políticas">
          <FieldRow label="Quién puede crear proyectos" error={message('whoCanCreateProjects')}>
            {(props) => (
              <Controller
                name="whoCanCreateProjects"
                control={control}
                render={({ field }) => (
                  <Select {...field} {...props} options={WHO_CAN_OPTIONS} disabled={!canEdit} />
                )}
              />
            )}
          </FieldRow>
          <FieldRow label="Quién puede crear equipos" error={message('whoCanCreateTeams')}>
            {(props) => (
              <Controller
                name="whoCanCreateTeams"
                control={control}
                render={({ field }) => <Select {...field} {...props} options={WHO_CAN_OPTIONS} disabled={!canEdit} />}
              />
            )}
          </FieldRow>
          <FieldRow label="Visibilidad por defecto de los proyectos" error={message('defaultProjectVisibility')}>
            {(props) => (
              <Controller
                name="defaultProjectVisibility"
                control={control}
                render={({ field }) => (
                  <Select {...field} {...props} options={VISIBILITY_OPTIONS} disabled={!canEdit} />
                )}
              />
            )}
          </FieldRow>
          <div className={styles.infoRow}>
            <span className={styles.infoLabel}>Exigir verificación en dos pasos</span>
            <Lozenge tone="neutral">Desactivado</Lozenge>
            <Typography.Text type="secondary">Disponible próximamente</Typography.Text>
          </div>
        </Section>
      </div>

      <div id="org-time">
        <Section title="Tiempo" description="Jornada de referencia para estimaciones y calendario." ariaLabel="Tiempo">
          <FieldRow label="Horas por día" required error={message('hoursPerDay')}>
            {(props) => (
              <Controller
                name="hoursPerDay"
                control={control}
                render={({ field }) => (
                  <InputNumber
                    {...field}
                    {...props}
                    min={1}
                    max={24}
                    disabled={!canEdit}
                    className={styles.numberInput}
                  />
                )}
              />
            )}
          </FieldRow>
          <FieldRow label="Días por semana" required error={message('daysPerWeek')}>
            {(props) => (
              <Controller
                name="daysPerWeek"
                control={control}
                render={({ field }) => (
                  <InputNumber
                    {...field}
                    {...props}
                    min={1}
                    max={7}
                    precision={0}
                    disabled={!canEdit}
                    className={styles.numberInput}
                  />
                )}
              />
            )}
          </FieldRow>
          <FieldRow label="Zona horaria" required error={message('timezone')}>
            {(props) => (
              <Controller
                name="timezone"
                control={control}
                render={({ field }) => (
                  <Select
                    {...field}
                    {...props}
                    showSearch
                    disabled={!canEdit}
                    options={timezones.map((zone) => ({ value: zone, label: zone }))}
                  />
                )}
              />
            )}
          </FieldRow>
        </Section>
      </div>

      <div className={styles.actions}>
        <Button onClick={() => reset(values)} disabled={!dirtyAndEditable || saving}>
          Descartar cambios
        </Button>
        <Button type="primary" htmlType="submit" loading={saving} disabled={!dirtyAndEditable}>
          Guardar cambios
        </Button>
      </div>
    </form>
  )
}
