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
  const fetchProjects = useProjectsStore((state) => state.fetchProjects)
  const createProject = useProjectsStore((state) => state.createProject)
  const fetchMembers = useProjectsStore((state) => state.fetchMembers)
  const addMember = useProjectsStore((state) => state.addMember)
  const updateMember = useProjectsStore((state) => state.updateMember)
  const removeMember = useProjectsStore((state) => state.removeMember)
  const [open, setOpen] = useState(false)
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [memberUserId, setMemberUserId] = useState('')
  const [memberRole, setMemberRole] = useState<'lead' | 'contributor' | 'viewer'>('contributor')
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
