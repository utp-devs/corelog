import { provideHttpClient } from '@angular/common/http';
import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { IssueAttachment } from '../../models/issue.model';
import { AttachmentApiService } from './attachment-api.service';

const dto = {
  id: 'att-1',
  issue_id: 'issue-1',
  uploader_id: 'user-1',
  file_name: 'nota.txt',
  content_type: 'text/plain',
  size_bytes: 3,
  created_at: 'a',
};

describe('AttachmentApiService', () => {
  let service: AttachmentApiService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(AttachmentApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  it('lists the attachments of an issue mapped to camelCase', () => {
    let items: IssueAttachment[] = [];
    service.list('issue-1').subscribe((result) => (items = result));
    httpMock.expectOne('/api/v1/issues/issue-1/attachments').flush({ data: [dto] });

    expect(items[0]).toEqual({
      id: 'att-1',
      issueId: 'issue-1',
      uploaderId: 'user-1',
      fileName: 'nota.txt',
      contentType: 'text/plain',
      sizeBytes: 3,
      createdAt: 'a',
    });
  });

  it('uploads the file as multipart form data', () => {
    service.upload('issue-1', new File(['abc'], 'nota.txt', { type: 'text/plain' })).subscribe();

    const request = httpMock.expectOne('/api/v1/issues/issue-1/attachments');
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toBeInstanceOf(FormData);
    expect((request.request.body as FormData).get('file')).toBeInstanceOf(File);
    request.flush({ data: dto });
  });

  it('downloads the file as a blob and releases the object URL', () => {
    const created: string[] = [];
    const revoked: string[] = [];
    const view = window;
    const originalCreate = view.URL.createObjectURL;
    const originalRevoke = view.URL.revokeObjectURL;
    view.URL.createObjectURL = (blob: Blob | MediaSource) => {
      if (blob instanceof Blob) {
        created.push(String(blob.size));
      }
      return 'blob:corelog-test';
    };
    view.URL.revokeObjectURL = (url: string) => revoked.push(url);
    const clicks: string[] = [];
    const originalClick = HTMLAnchorElement.prototype.click;
    HTMLAnchorElement.prototype.click = function (this: HTMLAnchorElement) {
      clicks.push(this.download);
    };

    try {
      const attachment: IssueAttachment = {
        id: 'att-1',
        issueId: 'issue-1',
        uploaderId: 'user-1',
        fileName: 'nota.txt',
        contentType: 'text/plain',
        sizeBytes: 3,
        createdAt: 'a',
      };
      service.download('issue-1', attachment).subscribe();

      const request = httpMock.expectOne('/api/v1/issues/issue-1/attachments/att-1');
      expect(request.request.responseType).toBe('blob');
      request.flush(new Blob(['abc']));

      expect(created).toEqual(['3']);
      expect(revoked).toEqual(['blob:corelog-test']);
      expect(clicks).toEqual(['nota.txt']);
    } finally {
      HTMLAnchorElement.prototype.click = originalClick;
      view.URL.createObjectURL = originalCreate;
      view.URL.revokeObjectURL = originalRevoke;
    }
  });

  it('lists silently: a failed read is shown by the page, not as a toast', () => {
    service.list('issue-1').subscribe({ error: () => undefined });
    const request = httpMock.expectOne('/api/v1/issues/issue-1/attachments');
    expect(request.request.context.get(SKIP_ERROR_NOTIFICATION)).toBe(true);
    request.flush({}, { status: 403, statusText: 'Forbidden' });
  });
});
