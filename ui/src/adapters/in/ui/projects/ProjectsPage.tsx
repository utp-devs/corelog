import { App, Button, Card, Empty, Form, Input, List, Modal, Select, Skeleton, Space, Typography } from 'antd'
import { useEffect, useState } from 'react'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import { PageHeader } from '@/shared/ui/PageHeader'
import styles from './ProjectsPage.module.css'

export function ProjectsPage() {
  const { message } = App.useApp()
  const user = useAuthStore((state) => state.user)
  const projects = useProjectsStore((state) => state.projects)
  const status = useProjectsStore((state) => state.status)
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
  const [memberUserId, setMemberUserId] = useState('')
  const [memberRole, setMemberRole] = useState<'lead' | 'contributor' | 'viewer'>('contributor')
  const [labelName, setLabelName] = useState('')
  const [labelColor, setLabelColor] = useState('blue')
  const [componentName, setComponentName] = useState('')
  const [componentDescription, setComponentDescription] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

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
        leadId: user?.id ?? '',
        visibility: values.visibility,
      })
      message.success('Proyecto creado')
      form.resetFields()
      setOpen(false)
    } catch (error) {
      if (error instanceof Error) message.error(error.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div>
      <PageHeader
        title="Proyectos"
        description="Organiza el trabajo en espacios independientes."
        actions={
          <Button type="primary" onClick={() => setOpen(true)} disabled={user?.role !== 'admin'}>
            Nuevo proyecto
          </Button>
        }
      />

      {status === 'loading' ? (
        <Skeleton active />
      ) : projects.length === 0 ? (
        <Empty description="Aun no hay proyectos" />
      ) : (
        <div className={styles.grid}>
          {projects.map((project) => (
            <Card
              key={project.id}
              title={`${project.key} - ${project.name}`}
              hoverable
              onClick={() => {
                setSelectedProjectId(project.id)
                void fetchMembers(project.id)
                void fetchLabels(project.id)
                void fetchComponents(project.id)
              }}
            >
              <p className={styles.description}>{project.description || 'Sin descripcion'}</p>
              <span className={styles.meta}>{project.type === 'scrum' ? 'Scrum' : 'Kanban'} · {project.visibility === 'private' ? 'Privado' : 'Equipo'}</span>
            </Card>
          ))}
        </div>
      )}

      {selectedProjectId && (
        <Card title="Miembros del proyecto" className={styles.members}>
          <Space wrap>
            <Input
              value={memberUserId}
              onChange={(event) => setMemberUserId(event.target.value)}
              placeholder="UUID del usuario"
              style={{ width: 280 }}
            />
            <Select
              value={memberRole}
              onChange={setMemberRole}
              options={[
                { value: 'lead', label: 'Lider' },
                { value: 'contributor', label: 'Colaborador' },
                { value: 'viewer', label: 'Lector' },
              ]}
            />
            <Button
              type="primary"
              disabled={!memberUserId}
              onClick={async () => {
                try {
                  await addMember(selectedProjectId, memberUserId, memberRole)
                  setMemberUserId('')
                  message.success('Miembro agregado')
                } catch (error) {
                  message.error(error instanceof Error ? error.message : 'No se pudo agregar el miembro')
                }
              }}
            >
              Agregar
            </Button>
          </Space>

          {members.length === 0 ? (
            <Empty description="Aun no hay miembros" />
          ) : (
            <List
              dataSource={members}
              renderItem={(member) => (
                <List.Item
                  actions={[
                    <Select
                      key="role"
                      size="small"
                      value={member.role}
                      onChange={(role) => {
                        void updateMember(selectedProjectId, member.userId, role).catch((error) =>
                          message.error(error instanceof Error ? error.message : 'No se pudo cambiar el rol'),
                        )
                      }}
                      options={[
                        { value: 'lead', label: 'Lider' },
                        { value: 'contributor', label: 'Colaborador' },
                        { value: 'viewer', label: 'Lector' },
                      ]}
                    />,
                    <Button
                      key="remove"
                      danger
                      size="small"
                      onClick={() => {
                        void removeMember(selectedProjectId, member.userId).catch((error) =>
                          message.error(error instanceof Error ? error.message : 'No se pudo eliminar el miembro'),
                        )
                      }}
                    >
                      Eliminar
                    </Button>,
                  ]}
                >
                  <Typography.Text>{member.userId}</Typography.Text>
                </List.Item>
              )}
            />
          )}
        </Card>
      )}

      {selectedProjectId && (
        <Card title="Etiquetas y componentes" className={styles.members}>
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Space wrap>
              <Input value={labelName} onChange={(event) => setLabelName(event.target.value)} placeholder="Nueva etiqueta" style={{ width: 220 }} />
              <Select value={labelColor} onChange={setLabelColor} options={[
                { value: 'blue', label: 'Azul' },
                { value: 'green', label: 'Verde' },
                { value: 'gold', label: 'Dorado' },
                { value: 'red', label: 'Rojo' },
              ]} />
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
            </Space>
            <List
              size="small"
              dataSource={labels}
              locale={{ emptyText: 'Aun no hay etiquetas' }}
              renderItem={(label) => (
                <List.Item actions={[
                  <Button key="delete" danger size="small" onClick={() => void deleteLabel(selectedProjectId, label.id)}>
                    Eliminar
                  </Button>,
                ]}>
                  {label.name} ({label.color})
                </List.Item>
              )}
            />
            <Space wrap>
              <Input value={componentName} onChange={(event) => setComponentName(event.target.value)} placeholder="Nuevo componente" style={{ width: 220 }} />
              <Input value={componentDescription} onChange={(event) => setComponentDescription(event.target.value)} placeholder="Descripcion" style={{ width: 260 }} />
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
            </Space>
            <List
              size="small"
              dataSource={components}
              locale={{ emptyText: 'Aun no hay componentes' }}
              renderItem={(component) => (
                <List.Item actions={[
                  <Button key="delete" danger size="small" onClick={() => void deleteComponent(selectedProjectId, component.id)}>
                    Eliminar
                  </Button>,
                ]}>
                  {component.name}{component.description ? ` - ${component.description}` : ''}
                </List.Item>
              )}
            />
          </Space>
        </Card>
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
        <Form form={form} layout="vertical" initialValues={{ type: 'kanban', visibility: 'team' }}>
          <Form.Item name="key" label="Clave" rules={[{ required: true, min: 1, max: 20 }]}>
            <Input placeholder="CORE" />
          </Form.Item>
          <Form.Item name="name" label="Nombre" rules={[{ required: true, min: 2, max: 120 }]}>
            <Input placeholder="Nombre del proyecto" />
          </Form.Item>
          <Form.Item name="description" label="Descripcion">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="type" label="Tipo" rules={[{ required: true }]}>
            <Select options={[{ value: 'kanban', label: 'Kanban' }, { value: 'scrum', label: 'Scrum' }]} />
          </Form.Item>
          <Form.Item name="visibility" label="Visibilidad" rules={[{ required: true }]}>
            <Select options={[{ value: 'team', label: 'Equipo' }, { value: 'private', label: 'Privado' }]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
