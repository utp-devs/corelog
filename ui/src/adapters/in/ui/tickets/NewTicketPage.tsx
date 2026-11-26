import { App } from 'antd'
import { useNavigate } from 'react-router-dom'

import type { Ticket } from '@/core/tickets/domain/ticket'
import { Section } from '@/shared/ui/Section'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { useCreateTicket } from './hooks/useCreateTicket'
import { searchMentionUsers } from './mentions'
import { TicketForm } from './TicketForm'

export function NewTicketPage() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const { submit, isSubmitting } = useCreateTicket()

  const handleSubmit = async (input: Parameters<typeof submit>[0]): Promise<Ticket> => {
    try {
      return await submit(input)
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'No se pudo crear la incidencia')
      throw error
    }
  }

  const handleCreated = (ticket: Ticket) => {
    message.success('Incidencia creada')
    navigate(`/issues/${ticket.id}`)
  }

  return (
    <PageContainer variant="reading" pageName="issue-new" fill>
      <PageHeader title="Nueva incidencia" description="Registra un nuevo incidente." />
      <Section fill>
        <TicketForm
          onSubmit={handleSubmit}
          onCreated={handleCreated}
          onCancel={() => navigate('/issues')}
          isSubmitting={isSubmitting}
          searchUsers={searchMentionUsers}
        />
      </Section>
    </PageContainer>
  )
}
