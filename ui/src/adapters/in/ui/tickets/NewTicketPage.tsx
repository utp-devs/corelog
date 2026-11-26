import { App } from 'antd'
import { useNavigate } from 'react-router-dom'

import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { useCreateTicket } from './hooks/useCreateTicket'
import { searchMentionUsers } from './mentions'
import { TicketForm } from './TicketForm'

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
    <PageContainer variant="reading" pageName="issue-new">
      <PageHeader title="Nueva incidencia" description="Registra un nuevo incidente." />
      <TicketForm
        onSubmit={handleSubmit}
        onCancel={() => navigate('/issues')}
        isSubmitting={isSubmitting}
        searchUsers={searchMentionUsers}
      />
    </PageContainer>
  )
}
