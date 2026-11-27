import { HttpClient } from '@angular/common/http';
import { DOCUMENT, Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { IssueAttachment } from '../../models/ticket.model';
import { silentRequest } from '../../utils/silent-request';

interface AttachmentDto {
  id: string;
  issue_id: string;
  uploader_id: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
}

interface Envelope<T> {
  data: T;
}

function toAttachment(dto: AttachmentDto): IssueAttachment {
  return {
    id: dto.id,
    issueId: dto.issue_id,
    uploaderId: dto.uploader_id,
    fileName: dto.file_name,
    contentType: dto.content_type,
    sizeBytes: dto.size_bytes,
    createdAt: dto.created_at,
  };
}

@Injectable({ providedIn: 'root' })
export class AttachmentApiService {
  private readonly http = inject(HttpClient);
  private readonly document = inject(DOCUMENT);
  private readonly baseUrl = `${inject(API_URL)}/issues`;

  list(issueId: string): Observable<IssueAttachment[]> {
    return this.http
      .get<Envelope<AttachmentDto[]>>(`${this.baseUrl}/${issueId}/attachments`, {
        context: silentRequest(),
      })
      .pipe(map((body) => body.data.map(toAttachment)));
  }

  /** Uploads one file as multipart form data. The browser sets the multipart boundary. */
  upload(issueId: string, file: File): Observable<IssueAttachment> {
    const body = new FormData();
    body.append('file', file);
    return this.http
      .post<Envelope<AttachmentDto>>(`${this.baseUrl}/${issueId}/attachments`, body)
      .pipe(map((response) => toAttachment(response.data)));
  }

  /** Fetches the file and saves it through a temporary object URL, as a browser download. */
  download(issueId: string, attachment: IssueAttachment): Observable<void> {
    return this.http
      .get(`${this.baseUrl}/${issueId}/attachments/${attachment.id}`, { responseType: 'blob' })
      .pipe(
        map((blob) => {
          const view = this.document.defaultView;
          if (!view) {
            return;
          }
          const url = view.URL.createObjectURL(blob);
          const link = this.document.createElement('a');
          link.href = url;
          link.download = attachment.fileName;
          link.click();
          view.URL.revokeObjectURL(url);
        }),
      );
  }
}
