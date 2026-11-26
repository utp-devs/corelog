import { PlusOutlined, ProjectOutlined } from '@ant-design/icons'
import { App, Button, Form, Input, Modal, Segmented, Select, Skeleton, Tooltip } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import type { Project } from '@/core/projects/domain/project'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { ErrorState } from '@/shared/ui/ErrorState'
import { useOrgMembersById } from '@/adapters/in/ui/organizations/useOrgMembersById'
import { Lozenge } from '@/shared/ui/Lozenge'
import { CardGrid } from '@/shared/ui/CardGrid'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { ProjectAvatar } from '@/shared/ui/ProjectAvatar'
import { Section } from '@/shared/ui/Section'
import { SearchInput } from '@/shared/ui/SearchInput'
import { Toolbar } from '@/shared/ui/Toolbar'
import { leadNameOf } from './projectLead'
import { accessVariantOf } from './projectLoadError'
import styles from './ProjectsPage.module.css'

type VisibilityFilter = 'all' | 'open' | 'private'
type ArchivedFilter = 'active' | 'archived' | 'all'
type MemberRole = 'lead' | 'contributor' | 'viewer'

const visibilityLabels: Record<Project['visibility'], string> = { open: 'Abierto', private: 'Privado' }
const typeLabels: Record<Project['type'], string> = { kanban: 'Kanban', scrum: 'Scrum' }
const roleOptions: { value: MemberRole; label: string }[] = [
  { value: 'lead', label: 'Líder' },
  { value: 'contributor', label: 'Colaborador' },
  { value: 'viewer', label: 'Lector' },
]

/** Filtra por texto (nombre o clave), visibilidad y estado de archivado. Sin peticiones: trabaja sobre la lista cargada. */
function filterProjects(projects: Project[], query: string, visibility: VisibilityFilter, archived: ArchivedFilter): Project[] {
  const term = query.trim().toLowerCase()
  return projects.filter((project) => {
    if (visibility !== 'all' && project.visibility !== visibility) return false
    const isArchived = project.archivedAt !== null
    if (archived === 'active' && isArchived) return false
    if (archived === 'archived' && !isArchived) return false
    if (term && !`${project.name} ${project.key}`.toLowerCase().includes(term)) return false
    return true
  })
}

export function ProjectsPage() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const user = useAuthStore((state) => state.user)
  const projects = useProjectsStore((state) => state.projects)
  const status = useProjectsStore((state) => state.status)
  const error = useProjectsStore((state) => state.error)
  const members = useProjectsStore((state) => state.members)
  const labels = useProjectsStore((state) => state.labels)
  const components = useProjectsStore((state) => state.components)
  const fetchProjects = useProjectsStore((state) => state.fetchProjects)
  const createProject = useProjectsStore((state) => state.createProject)
  const fetchMembers = useProjectsStore((state) => state.fetchMembers)
  const addMember = useProjectsStore((state) => state.addMember)
  const updateMember = useProjectsStore((state) => state.updateMember)
  const removeMember = useProjectsStore((state) => state.removeMember)
  const fetchLabels = useProjectsStore((state) => state.fetchLabels)
  const createLabel = useProjectsStore((state) => state.createLabel)
  const deleteLabel = useProjectsStore((state) => state.deleteLabel)
  const fetchComponents = useProjectsStore((state) => state.fetchComponents)
  const createComponent = useProjectsStore((state) => state.createComponent)
  const deleteComponent = useProjectsStore((state) => state.deleteComponent)
  const [open, setOpen] = useState(false)
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [visibility, setVisibility] = useState<VisibilityFilter>('all')
  const [archived, setArchived] = useState<ArchivedFilter>('active')
  const [memberUserId, setMemberUserId] = useState('')
  const [memberRole, setMemberRole] = useState<MemberRole>('contributor')
  const [labelName, setLabelName] = useState('')
  const [labelColor, setLabelColor] = useState('blue')
  const [componentName, setComponentName] = useState('')
  const [componentDescription, setComponentDescription] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const isAdmin = user?.role === 'admin'
  const selectedProject = projects.find((project) => project.id === selectedProjectId)
  const visible = useMemo(() => filterProjects(projects, query, visibility, archived), [projects, query, visibility, archived])
  // All projects of the list belong to the current organization: its members resolve the leads.
  const { members: orgMembers, status: orgMembersStatus } = useOrgMembersById(projects[0]?.organizationId)

  useEffect(() => {
    void fetchProjects()
  }, [fetchProjects])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await createProject({
        key: values.key,
        name: values.name,
        description: values.description ?? '',
        type: values.type,
        visibility: values.visibility,
      })
      message.success('Proyecto creado')
      form.resetFields()
      setOpen(false)
    } catch (failure) {
      if (failure instanceof Error) message.error(failure.message)
    } finally {
      setSubmitting(false)
    }
  }

  const selectProject = (project: Project) => {
    setSelectedProjectId(project.id)
    void fetchMembers(project.id)
    void fetchLabels(project.id)
    void fetchComponents(project.id)
  }

  const accessVariant = accessVariantOf(error)
  const loading = status === 'loading' && projects.length === 0

  if (accessVariant !== null && projects.length === 0) {
    return (
      <PageContainer variant="wide" pageName="projects">
        <ErrorPage variant={accessVariant} />
      </PageContainer>
    )
  }

  let body
  if (loading) {
    body = (
      <ul className={styles.grid} aria-label="Cargando proyectos" aria-busy="true">
        {[0, 1, 2].map((index) => (
          <li key={index} className={styles.cardItem}>
            <Section>
              <Skeleton active paragraph={{ rows: 2 }} />
            </Section>
          </li>
        ))}
      </ul>
    )
  } else if (status === 'error' && projects.length === 0) {
    body = <ErrorState message={error?.message ?? 'No se pudieron cargar los proyectos.'} onRetry={() => void fetchProjects()} />
  } else if (projects.length === 0) {
    body = (
      <Section fill>
        <EmptyState
          title="Aún no hay proyectos"
          description="Crea el primero para organizar el trabajo en espacios independientes."
          action={isAdmin ? <Button type="primary" onClick={() => setOpen(true)}>Nuevo proyecto</Button> : undefined}
        />
      </Section>
    )
  } else if (visible.length === 0) {
    body = (
      <Section fill>
        <EmptyState title="Sin resultados" description="Ningún proyecto coincide con los filtros. Cambia la búsqueda o el filtro de estado." />
      </Section>
    )
  } else {
    body = (
      <CardGrid ariaLabel="Proyectos" minCardWidth={280} fill>
        {visible.map((project) => (
          <article key={project.id} className={styles.card} data-testid={`project-card-${project.key}`}>
              <header className={styles.cardHeader}>
                <ProjectAvatar name={project.name} projectKey={project.key} size={40} />
                <div className={styles.cardTitle}>
                  <h3 className={styles.name} title={project.name}>{project.name}</h3>
                  <span className={styles.key}>{project.key}</span>
                </div>
              </header>
              <p className={styles.description} title={project.description || undefined}>
                {project.description || 'Sin descripción'}
              </p>
              <p className={styles.lead}>
                Responsable: <span data-testid={`project-lead-${project.key}`}>{leadNameOf(project.leadId, orgMembers, orgMembersStatus)}</span>
              </p>
              <div className={styles.tags}>
                <Lozenge tone={project.visibility === 'open' ? 'info' : 'warning'}>{visibilityLabels[project.visibility]}</Lozenge>
                <Lozenge>{typeLabels[project.type]}</Lozenge>
                {project.archivedAt && <Lozenge tone="neutral">Archivado</Lozenge>}
              </div>
              <footer className={styles.cardActions}>
                <Button type="link" icon={<ProjectOutlined aria-hidden="true" />} onClick={() => navigate(`/projects/${project.id}/board`)}>
                  Abrir tablero
                </Button>
                <Button type="link" onClick={() => selectProject(project)} aria-pressed={selectedProjectId === project.id}>
                  Miembros y etiquetas
                </Button>
              </footer>
          </article>
        ))}
      </CardGrid>
    )
  }

  return (
    <PageContainer variant="wide" pageName="projects" ready={!loading && status !== 'loading'} fill>
      <PageHeader
        title="Proyectos"
        description="Organiza el trabajo en espacios independientes."
        actions={
          <Tooltip title={isAdmin ? undefined : 'Solo los administradores pueden crear proyectos.'}>
            <span>
              <Button type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={() => setOpen(true)} disabled={!isAdmin}>
                Nuevo proyecto
              </Button>
            </span>
          </Tooltip>
        }
      />

      <Toolbar
        left={
          <div className={styles.filters}>
            <SearchInput value={query} onChange={setQuery} placeholder="Buscar por nombre o clave" ariaLabel="Buscar proyectos" />
            <Segmented
              aria-label="Filtrar por visibilidad"
              value={visibility}
              onChange={(value) => setVisibility(value as VisibilityFilter)}
              options={[
                { label: 'Todos', value: 'all' },
                { label: 'Abiertos', value: 'open' },
                { label: 'Privados', value: 'private' },
              ]}
            />
            <Select<ArchivedFilter>
              aria-label="Filtrar por estado"
              value={archived}
              onChange={setArchived}
              className={styles.archivedFilter}
              options={[
                { value: 'active', label: 'Activos' },
                { value: 'archived', label: 'Archivados' },
                { value: 'all', label: 'Todos los estados' },
              ]}
            />
          </div>
        }
      />

      {body}

      {selectedProjectId && (
        <Section
          title={selectedProject ? `Miembros de ${selectedProject.name}` : 'Miembros del proyecto'}
          description="Líder, colaborador o lector. Los cambios se aplican al momento."
        >
          <div className={styles.inlineForm}>
            <Input
              value={memberUserId}
              onChange={(event) => setMemberUserId(event.target.value)}
              placeholder="UUID del usuario"
              aria-label="UUID del usuario"
              className={styles.memberInput}
            />
            <Select value={memberRole} onChange={setMemberRole} options={roleOptions} aria-label="Rol del nuevo miembro" />
            <Button
              type="primary"
              disabled={!memberUserId}
              onClick={async () => {
                try {
                  await addMember(selectedProjectId, memberUserId, memberRole)
                  setMemberUserId('')
                  message.success('Miembro agregado')
                } catch (failure) {
                  message.error(failure instanceof Error ? failure.message : 'No se pudo agregar el miembro')
                }
              }}
            >
              Agregar
            </Button>
          </div>

          {members.length === 0 ? (
            <EmptyState title="Aún no hay miembros" description="Agrega a un usuario con su UUID para empezar." />
          ) : (
            <ul className={styles.itemList} aria-label="Miembros del proyecto">
              {members.map((member) => (
                <li key={member.userId} className={styles.item}>
                  <span className={styles.itemText} title={member.userId}>{member.userId}</span>
                  <div className={styles.itemActions}>
                    <Select
                      size="small"
                      value={member.role}
                      aria-label={`Rol de ${member.userId}`}
                      onChange={(role: MemberRole) => {
                        void updateMember(selectedProjectId, member.userId, role).catch((failure: unknown) =>
                          message.error(failure instanceof Error ? failure.message : 'No se pudo cambiar el rol'),
                        )
                      }}
                      options={roleOptions}
                    />
                    <Button
                      danger
                      size="small"
                      onClick={() => {
                        void removeMember(selectedProjectId, member.userId).catch((failure: unknown) =>
                          message.error(failure instanceof Error ? failure.message : 'No se pudo eliminar el miembro'),
                        )
                      }}
                    >
                      Eliminar
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Section>
      )}

      {selectedProjectId && (
        <Section title="Etiquetas y componentes" description="Organizan las incidencias dentro del proyecto.">
          <div className={styles.twoColumns}>
            <div className={styles.column}>
              <h3 className={styles.subtitle}>Etiquetas</h3>
              <div className={styles.inlineForm}>
                <Input value={labelName} onChange={(event) => setLabelName(event.target.value)} placeholder="Nueva etiqueta" aria-label="Nombre de la etiqueta" className={styles.labelInput} />
                <Select
                  value={labelColor}
                  onChange={setLabelColor}
                  aria-label="Color de la etiqueta"
                  options={[
                    { value: 'blue', label: 'Azul' },
                    { value: 'green', label: 'Verde' },
                    { value: 'gold', label: 'Dorado' },
                    { value: 'red', label: 'Rojo' },
                  ]}
                />
                <Button
                  type="primary"
                  disabled={!labelName.trim()}
                  onClick={async () => {
                    await createLabel(selectedProjectId, labelName, labelColor)
                    setLabelName('')
                  }}
                >
                  Crear etiqueta
                </Button>
              </div>
              {labels.length === 0 ? (
                <EmptyState title="Aún no hay etiquetas" />
              ) : (
                <ul className={styles.itemList} aria-label="Etiquetas del proyecto">
                  {labels.map((label) => (
                    <li key={label.id} className={styles.item}>
                      <span className={styles.itemText}>{label.name} ({label.color})</span>
                      <Button danger size="small" onClick={() => void deleteLabel(selectedProjectId, label.id)}>Eliminar</Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className={styles.column}>
              <h3 className={styles.subtitle}>Componentes</h3>
              <div className={styles.inlineForm}>
                <Input value={componentName} onChange={(event) => setComponentName(event.target.value)} placeholder="Nuevo componente" aria-label="Nombre del componente" className={styles.componentNameInput} />
                <Input value={componentDescription} onChange={(event) => setComponentDescription(event.target.value)} placeholder="Descripción" aria-label="Descripción del componente" className={styles.componentDescriptionInput} />
                <Button
                  type="primary"
                  disabled={!componentName.trim()}
                  onClick={async () => {
                    await createComponent(selectedProjectId, componentName, componentDescription)
                    setComponentName('')
                    setComponentDescription('')
                  }}
                >
                  Crear componente
                </Button>
              </div>
              {components.length === 0 ? (
                <EmptyState title="Aún no hay componentes" />
              ) : (
                <ul className={styles.itemList} aria-label="Componentes del proyecto">
                  {components.map((component) => (
                    <li key={component.id} className={styles.item}>
                      <span className={styles.itemText}>{component.name}{component.description ? ` - ${component.description}` : ''}</span>
                      <Button danger size="small" onClick={() => void deleteComponent(selectedProjectId, component.id)}>Eliminar</Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </Section>
      )}

      <Modal
        open={open}
        title="Nuevo proyecto"
        onCancel={() => setOpen(false)}
        onOk={handleCreate}
        confirmLoading={submitting}
        okText="Crear"
        cancelText="Cancelar"
      >
        <Form form={form} layout="vertical" initialValues={{ type: 'kanban', visibility: 'open' }}>
          <Form.Item name="key" label="Clave" rules={[{ required: true, min: 1, max: 20 }]}>
            <Input placeholder="CORE" />
          </Form.Item>
          <Form.Item name="name" label="Nombre" rules={[{ required: true, min: 2, max: 120 }]}>
            <Input placeholder="Nombre del proyecto" />
          </Form.Item>
          <Form.Item name="description" label="Descripción">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="type" label="Tipo" rules={[{ required: true }]}>
            <Select options={[{ value: 'kanban', label: 'Kanban' }, { value: 'scrum', label: 'Scrum' }]} />
          </Form.Item>
          <Form.Item name="visibility" label="Visibilidad" rules={[{ required: true }]}>
            <Select options={[{ value: 'open', label: 'Abierto' }, { value: 'private', label: 'Privado' }]} />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  )
}
