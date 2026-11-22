import { apiClient } from './api-client'
import type { IssueAttachment } from '@/core/tickets/domain/attachment'

interface AttachmentDto {
  id: string; issue_id: string; uploader_id: string; file_name: string
  content_type: string; size_bytes: number; created_at: string
}

function fromDto(dto: AttachmentDto): IssueAttachment {
  return { id: dto.id, issueId: dto.issue_id, uploaderId: dto.uploader_id, fileName: dto.file_name,
    contentType: dto.content_type, sizeBytes: dto.size_bytes, createdAt: dto.created_at }
}

export class AttachmentHttpRepository {
  async list(issueId: string): Promise<IssueAttachment[]> {
    const { data } = await apiClient.get<{ data: AttachmentDto[] }>(`/issues/${issueId}/attachments`)
    return data.data.map(fromDto)
  }

  async upload(issueId: string, file: File): Promise<IssueAttachment> {
    const body = new FormData()
    body.append('file', file)
    const { data } = await apiClient.post<{ data: AttachmentDto }>(`/issues/${issueId}/attachments`, body, {
      headers: { 'Content-Type': undefined },
    })
    return fromDto(data.data)
  }

  async download(issueId: string, attachment: IssueAttachment): Promise<void> {
    const { data } = await apiClient.get<Blob>(`/issues/${issueId}/attachments/${attachment.id}`, { responseType: 'blob' })
    const url = URL.createObjectURL(data)
    const link = document.createElement('a')
    link.href = url
    link.download = attachment.fileName
    link.click()
    URL.revokeObjectURL(url)
  }
}
