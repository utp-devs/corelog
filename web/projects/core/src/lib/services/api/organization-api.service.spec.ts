import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { asApiError } from '../../utils/api-error';
import { OrganizationApiService } from './organization-api.service';

const BASE = '/api/v1';
const ORG_ID = '11111111-1111-1111-1111-111111111111';

const orgDto = {
  id: ORG_ID,
  slug: 'acme',
  name: 'Acme',
  description: '',
  settings: {
    who_can_create_projects: 'members',
    who_can_create_teams: 'admins',
    default_project_visibility: 'open',
    require_2fa: false,
    max_upload_bytes: 10485760,
    hours_per_day: 8,
    days_per_week: 5,
    timezone: 'Europe/Madrid',
  },
  role: 'admin',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-02T00:00:00Z',
};

const memberDto = {
  user_id: 'u-2',
  name: 'Luis Pardo',
  email: 'luis@acme.test',
  role: 'member',
  joined_at: '2026-02-01T00:00:00Z',
};

const invitationDto = {
  id: 'i-1',
  email: 'ana@acme.test',
  role: 'admin',
  status: 'pending',
  invited_by: 'u-1',
  expires_at: '2026-12-01T00:00:00Z',
  accepted_at: null,
  revoked_at: null,
  created_at: '2026-11-20T00:00:00Z',
};

describe('OrganizationApiService', () => {
  let service: OrganizationApiService;
  let controller: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(OrganizationApiService);
    controller = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    controller.verify();
  });

  describe('organizations and invitations', () => {
    it('lists organizations and maps settings to camelCase', async () => {
      const pending = firstValueFrom(service.list());
      const req = controller.expectOne(`${BASE}/organizations`);
      expect(req.request.method).toBe('GET');
      req.flush({ data: [orgDto] });
      const orgs = await pending;
      expect(orgs[0]).toMatchObject({
        id: ORG_ID,
        slug: 'acme',
        role: 'admin',
        createdAt: '2026-01-01T00:00:00Z',
      });
      expect(orgs[0].settings).toEqual({
        whoCanCreateProjects: 'members',
        whoCanCreateTeams: 'admins',
        defaultProjectVisibility: 'open',
        require2fa: false,
        maxUploadBytes: 10485760,
        hoursPerDay: 8,
        daysPerWeek: 5,
        timezone: 'Europe/Madrid',
      });
      expect(orgs[0].hasLogo).toBe(false);
    });

    it('creates an organization without empty optional fields and defaults the role to owner', async () => {
      const pending = firstValueFrom(service.create({ name: 'Acme' }));
      const req = controller.expectOne(`${BASE}/organizations`);
      expect(req.request.body).toEqual({ name: 'Acme' });
      req.flush({ data: { ...orgDto, role: undefined } }, { status: 201, statusText: 'Created' });
      expect((await pending).role).toBe('owner');
    });

    it('sends the slug when given', async () => {
      const pending = firstValueFrom(service.create({ name: 'Acme', slug: 'acme' }));
      const req = controller.expectOne(`${BASE}/organizations`);
      expect(req.request.body).toEqual({ name: 'Acme', slug: 'acme' });
      req.flush({ data: orgDto }, { status: 201, statusText: 'Created' });
      await pending;
    });

    it('posts all invitations of an organization in one request and keeps the plain token', async () => {
      const invitation = {
        ...invitationDto,
        id: 'i1',
        email: 'ana@example.com',
        role: 'member',
        expires_at: '2026-01-08T00:00:00Z',
        token: 'tok-abcdefghijklmnop',
        accept_url: 'http://localhost/invite/tok-abcdefghijklmnop',
      };
      const pending = firstValueFrom(
        service.createInvitations(ORG_ID, [{ email: 'ana@example.com', role: 'member' }]),
      );
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/invitations`);
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({
        invitations: [{ email: 'ana@example.com', role: 'member' }],
      });
      req.flush({ data: { invitations: [invitation] } }, { status: 201, statusText: 'Created' });
      const created = await pending;
      expect(created[0]).toMatchObject({
        token: 'tok-abcdefghijklmnop',
        acceptUrl: invitation.accept_url,
        status: 'pending',
        expiresAt: '2026-01-08T00:00:00Z',
      });
    });

    it('previews a token with a public GET and maps the preview', async () => {
      const pending = firstValueFrom(service.previewInvitation('tok-1'));
      const req = controller.expectOne(`${BASE}/invitations/tok-1`);
      expect(req.request.method).toBe('GET');
      req.flush({
        data: {
          organization_id: 'org-1',
          organization_name: 'Acme',
          has_logo: true,
          invited_by: 'Ana Owner',
          role: 'guest',
          email: 'ana@example.com',
          status: 'pending',
          expires_at: '2026-01-08T00:00:00Z',
        },
      });
      expect(await pending).toEqual({
        organizationId: 'org-1',
        organizationName: 'Acme',
        hasLogo: true,
        invitedBy: 'Ana Owner',
        role: 'guest',
        email: 'ana@example.com',
        status: 'pending',
        expiresAt: '2026-01-08T00:00:00Z',
      });
    });

    it('accepts an invitation and maps the membership', async () => {
      const pending = firstValueFrom(service.acceptInvitation('tok-1'));
      const req = controller.expectOne(`${BASE}/invitations/tok-1/accept`);
      expect(req.request.method).toBe('POST');
      req.flush(
        {
          data: {
            organization_id: ORG_ID,
            user_id: 'u1',
            role: 'member',
            joined_at: '2026-01-02T00:00:00Z',
          },
        },
        { status: 201, statusText: 'Created' },
      );
      expect(await pending).toEqual({
        organizationId: ORG_ID,
        userId: 'u1',
        role: 'member',
        joinedAt: '2026-01-02T00:00:00Z',
      });
    });

    it('leaves an organization with a POST to its leave endpoint', async () => {
      const pending = firstValueFrom(service.leave(ORG_ID), { defaultValue: undefined });
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/leave`);
      expect(req.request.method).toBe('POST');
      req.flush(null, { status: 204, statusText: 'No Content' });
      await pending;
    });
  });

  describe('administration', () => {
    it('sends only the changed fields of a settings update in snake_case', async () => {
      const pending = firstValueFrom(
        service.updateOrganization(ORG_ID, {
          name: 'Acme Labs',
          settings: { whoCanCreateProjects: 'admins', hoursPerDay: 7.5 },
        }),
      );
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}`);
      expect(req.request.method).toBe('PATCH');
      expect(req.request.body).toEqual({
        name: 'Acme Labs',
        settings: { who_can_create_projects: 'admins', hours_per_day: 7.5 },
      });
      req.flush({
        data: {
          ...orgDto,
          name: 'Acme Labs',
          settings: {
            ...orgDto.settings,
            who_can_create_projects: 'admins',
            default_project_visibility: 'private',
            hours_per_day: 7.5,
          },
        },
      });
      const org = await pending;
      expect(org.settings.hoursPerDay).toBe(7.5);
      expect(org.settings.defaultProjectVisibility).toBe('private');
    });

    it('uploads the logo as multipart and returns the logo URL', async () => {
      const file = new File(['png'], 'logo.png', { type: 'image/png' });
      const pending = firstValueFrom(service.uploadLogo(ORG_ID, file));
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/logo`);
      expect(req.request.method).toBe('PUT');
      expect(req.request.body).toBeInstanceOf(FormData);
      expect((req.request.body as FormData).get('file')).toBeInstanceOf(File);
      req.flush({ data: { logo_url: `/organizations/${ORG_ID}/logo` } });
      expect(await pending).toBe(`/organizations/${ORG_ID}/logo`);
    });

    it('returns null when the organization has no logo', async () => {
      const pending = firstValueFrom(service.getLogo(ORG_ID));
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/logo`);
      req.flush(new Blob(['{}']), { status: 404, statusText: 'Not Found' });
      expect(await pending).toBeNull();
    });

    it('fetchLogoBlob requests the logo as a blob and resolves null when the request fails', async () => {
      const blob = new Blob(['png-bytes'], { type: 'image/png' });
      const first = firstValueFrom(service.fetchLogoBlob(ORG_ID));
      const ok = controller.expectOne(`${BASE}/organizations/${ORG_ID}/logo`);
      expect(ok.request.method).toBe('GET');
      expect(ok.request.responseType).toBe('blob');
      ok.flush(blob);
      expect(await first).toEqual(blob);

      const second = firstValueFrom(service.fetchLogoBlob(ORG_ID));
      controller
        .expectOne(`${BASE}/organizations/${ORG_ID}/logo`)
        .flush(new Blob(['{}']), { status: 500, statusText: 'Error' });
      expect(await second).toBeNull();
    });

    it('getLogo re-throws failures other than 404 as ApiError', async () => {
      const pending = firstValueFrom(service.getLogo(ORG_ID)).catch((error: unknown) =>
        asApiError(error),
      );
      controller
        .expectOne(`${BASE}/organizations/${ORG_ID}/logo`)
        .flush(new Blob(['{}']), { status: 403, statusText: 'Forbidden' });
      expect(await pending).toMatchObject({ status: 403, code: 'http.403' });
    });

    it('deletes the logo with DELETE', async () => {
      const pending = firstValueFrom(service.deleteLogo(ORG_ID), { defaultValue: undefined });
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/logo`);
      expect(req.request.method).toBe('DELETE');
      req.flush(null, { status: 204, statusText: 'No Content' });
      await pending;
    });

    it('maps members to camelCase', async () => {
      const pending = firstValueFrom(service.listMembers(ORG_ID));
      controller.expectOne(`${BASE}/organizations/${ORG_ID}/members`).flush({ data: [memberDto] });
      expect(await pending).toEqual([
        {
          userId: 'u-2',
          name: 'Luis Pardo',
          displayName: '',
          email: 'luis@acme.test',
          jobTitle: '',
          department: '',
          hasAvatar: false,
          role: 'member',
          joinedAt: '2026-02-01T00:00:00Z',
        },
      ]);
    });

    it('changes a role with a PATCH on the member', async () => {
      const pending = firstValueFrom(service.changeMemberRole(ORG_ID, 'u-2', 'admin'), {
        defaultValue: undefined,
      });
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/members/u-2`);
      expect(req.request.method).toBe('PATCH');
      expect(req.request.body).toEqual({ role: 'admin' });
      req.flush(null, { status: 204, statusText: 'No Content' });
      await pending;
    });

    it('surfaces org.last_owner from a removal as an ApiError', async () => {
      const pending = firstValueFrom(service.removeMember(ORG_ID, 'u-1'), {
        defaultValue: undefined,
      }).catch((error: unknown) => asApiError(error));
      controller
        .expectOne(`${BASE}/organizations/${ORG_ID}/members/u-1`)
        .flush(
          { error: { code: 'org.last_owner', message: 'Es el último propietario' } },
          { status: 409, statusText: 'Conflict' },
        );
      const error = await pending;
      expect(error).toMatchObject({
        status: 409,
        code: 'org.last_owner',
        message: 'Es el último propietario',
      });
    });

    it('removes a member with DELETE on the member path', async () => {
      const pending = firstValueFrom(service.removeMember(ORG_ID, 'u-2'), {
        defaultValue: undefined,
      });
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/members/u-2`);
      expect(req.request.method).toBe('DELETE');
      req.flush(null, { status: 204, statusText: 'No Content' });
      await pending;
    });

    it('transfers ownership with the target user id', async () => {
      const pending = firstValueFrom(service.transferOwnership(ORG_ID, 'u-2'), {
        defaultValue: undefined,
      });
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/transfer-ownership`);
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({ user_id: 'u-2' });
      req.flush(null, { status: 204, statusText: 'No Content' });
      await pending;
    });

    it('sends the confirmation slug when deleting the organization', async () => {
      const pending = firstValueFrom(service.deleteOrganization(ORG_ID, 'acme'), {
        defaultValue: undefined,
      });
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}`);
      expect(req.request.method).toBe('DELETE');
      expect(req.request.body).toEqual({ confirm_slug: 'acme' });
      req.flush(null, { status: 204, statusText: 'No Content' });
      await pending;
    });

    it('lists invitations with the requested status scope', async () => {
      const pending = firstValueFrom(service.listInvitations(ORG_ID, 'all'));
      const req = controller.expectOne(
        (request) => request.url === `${BASE}/organizations/${ORG_ID}/invitations`,
      );
      expect(req.request.params.get('status')).toBe('all');
      req.flush({ data: [invitationDto] });
      const invitations = await pending;
      expect(invitations[0]).toMatchObject({
        id: 'i-1',
        invitedBy: 'u-1',
        expiresAt: '2026-12-01T00:00:00Z',
      });
    });

    it('revokes an invitation with DELETE', async () => {
      const pending = firstValueFrom(service.revokeInvitation(ORG_ID, 'i-1'), {
        defaultValue: undefined,
      });
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/invitations/i-1`);
      expect(req.request.method).toBe('DELETE');
      req.flush(null, { status: 204, statusText: 'No Content' });
      await pending;
    });

    it('resends an invitation and returns the new plain link', async () => {
      const pending = firstValueFrom(service.resendInvitation(ORG_ID, 'i-1'));
      const req = controller.expectOne(`${BASE}/organizations/${ORG_ID}/invitations/i-1/resend`);
      expect(req.request.method).toBe('POST');
      req.flush({
        data: {
          ...invitationDto,
          token: 'tok-123-abcdefghijklmnop',
          accept_url: 'https://corelog.test/invite/tok-123',
        },
      });
      expect(await pending).toMatchObject({
        token: 'tok-123-abcdefghijklmnop',
        acceptUrl: 'https://corelog.test/invite/tok-123',
      });
    });
  });
});
