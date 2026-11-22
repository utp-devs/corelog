import { App, Card } from 'antd'
import { useNavigate } from 'react-router-dom'

import { useCreateTicket } from './hooks/useCreateTicket'
import { TicketForm } from './TicketForm'
import styles from './NewTicketPage.module.css'
import { PageHeader } from '@/shared/ui/PageHeader'

export function NewTicketPage() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const { submit, isSubmitting } = useCreateTicket()

  const handleSubmit = async (input: Parameters<typeof submit>[0]) => {
    try {
      const ticket = await submit(input)
      message.success('Incidencia creada')
      navigate(`/issues/${ticket.id}`)
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'No se pudo crear la incidencia')
      throw error
    }
  }

  return (
    <div>
      <PageHeader title="Nueva incidencia" description="Registra un nuevo incidente." />
      <Card className={styles.card}>
        <TicketForm onSubmit={handleSubmit} isSubmitting={isSubmitting} />
      </Card>
    </div>
  )
}
