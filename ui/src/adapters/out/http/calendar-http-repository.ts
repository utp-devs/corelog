import type { CalendarEvent, CalendarEventKind } from '@/core/projects/domain/calendar'
import { apiClient } from './api-client'

interface CalendarEventDto {
  kind: CalendarEventKind
  date: string
  title: string
  source_id: string
  link: string
}

export class CalendarHttpRepository {
  // from y to en formato AAAA-MM-DD, ambos incluidos.
  async events(projectId: string, from: string, to: string): Promise<CalendarEvent[]> {
    const { data } = await apiClient.get<{ data: CalendarEventDto[] }>('/calendar/events', {
      params: { project_id: projectId, from, to },
    })
    return data.data.map((event) => ({
      kind: event.kind,
      date: event.date,
      title: event.title,
      sourceId: event.source_id,
      link: event.link,
    }))
  }
}
