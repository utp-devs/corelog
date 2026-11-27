import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, catchError, map, of, throwError } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION, SKIP_LOADING, publicRequest } from '../../config/http-context';
import {
  CreateOrganizationInput,
  Invitation,
  InvitationCreated,
  InvitationItemInput,
  InvitationPreview,
  InvitationScope,
  InvitationStatus,
  Organization,
  OrganizationMember,
  OrganizationMembership,
  OrganizationPatch,
  OrganizationSettings,
  OrgRole,
} from '../../models/organization.model';
import { asApiError } from '../../utils/api-error';
import { silentRequest } from '../../utils/silent-request';

interface SettingsDto {
  readonly who_can_create_projects: OrganizationSettings['whoCanCreateProjects'];
  readonly who_can_create_teams: OrganizationSettings['whoCanCreateTeams'];
  readonly default_project_visibility: OrganizationSettings['defaultProjectVisibility'];
  readonly require_2fa: boolean;
  readonly max_upload_bytes: number;
  readonly hours_per_day: number;
  readonly days_per_week: number;
  readonly timezone: string;
}

interface OrganizationDto {
  readonly id: string;
  readonly slug: string;
  readonly name: string;
  readonly description: string;
  readonly settings: SettingsDto;
  readonly role?: OrgRole;
  readonly has_logo?: boolean;
  readonly created_at: string;
  readonly updated_at: string;
}

interface InvitationDto {
  readonly id: string;
  readonly email: string;
  readonly role: OrgRole;
  readonly status: InvitationStatus;
  readonly invited_by: string | null;
  readonly expires_at: string;
  readonly accepted_at: string | null;
  readonly revoked_at: string | null;
  readonly created_at: string;
}

interface InvitationCreatedDto extends InvitationDto {
  readonly token: string;
  readonly accept_url: string;
}

interface InvitationPreviewDto {
  readonly organization_id: string;
  readonly organization_name: string;
  readonly has_logo: boolean;
  readonly invited_by: string | null;
  readonly role: OrgRole;
  readonly email: string;
  readonly status: InvitationStatus;
  readonly expires_at: string;
}

interface OrganizationMemberDto {
  readonly user_id: string;
  readonly name: string;
  readonly display_name?: string;
  readonly email: string;
  readonly job_title?: string;
  readonly department?: string;
  readonly has_avatar?: boolean;
  readonly role: OrgRole;
  readonly joined_at: string;
}

interface MembershipDto {
  readonly organization_id: string;
  readonly user_id: string;
  readonly role: OrgRole;
  readonly joined_at: string;
}

function toOrganization(dto: OrganizationDto): Organization {
  return {
    id: dto.id,
    slug: dto.slug,
    name: dto.name,
    description: dto.description,
    settings: {
      whoCanCreateProjects: dto.settings.who_can_create_projects,
      whoCanCreateTeams: dto.settings.who_can_create_teams,
      defaultProjectVisibility: dto.settings.default_project_visibility,
      require2fa: dto.settings.require_2fa,
      maxUploadBytes: dto.settings.max_upload_bytes,
      hoursPerDay: dto.settings.hours_per_day,
      daysPerWeek: dto.settings.days_per_week,
      timezone: dto.settings.timezone,
    },
    // The create response carries no role; the caller of POST /organizations is always its owner.
    role: dto.role ?? 'owner',
    hasLogo: dto.has_logo ?? false,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

function toInvitation(dto: InvitationDto): Invitation {
  return {
    id: dto.id,
    email: dto.email,
    role: dto.role,
    status: dto.status,
    invitedBy: dto.invited_by,
    expiresAt: dto.expires_at,
    acceptedAt: dto.accepted_at,
    revokedAt: dto.revoked_at,
    createdAt: dto.created_at,
  };
}

function toInvitationCreated(dto: InvitationCreatedDto): InvitationCreated {
  return { ...toInvitation(dto), token: dto.token, acceptUrl: dto.accept_url };
}

function toPreview(dto: InvitationPreviewDto): InvitationPreview {
  return {
    organizationId: dto.organization_id,
    organizationName: dto.organization_name,
    hasLogo: dto.has_logo,
    invitedBy: dto.invited_by,
    role: dto.role,
    email: dto.email,
    status: dto.status,
    expiresAt: dto.expires_at,
  };
}

function toMember(dto: OrganizationMemberDto): OrganizationMember {
  return {
    userId: dto.user_id,
    name: dto.name,
    displayName: dto.display_name ?? '',
    email: dto.email,
    jobTitle: dto.job_title ?? '',
    department: dto.department ?? '',
    hasAvatar: dto.has_avatar ?? false,
    role: dto.role,
    joinedAt: dto.joined_at,
  };
}

function toMembership(dto: MembershipDto): OrganizationMembership {
  return {
    organizationId: dto.organization_id,
    userId: dto.user_id,
    role: dto.role,
    joinedAt: dto.joined_at,
  };
}

/** Sends only the fields present in the patch, in snake_case (PATCH is a partial update). */
function toPatchBody(patch: OrganizationPatch): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  if (patch.name !== undefined) {
    body['name'] = patch.name;
  }
  if (patch.description !== undefined) {
    body['description'] = patch.description;
  }
  if (patch.settings) {
    const s = patch.settings;
    const settings: Record<string, unknown> = {};
    if (s.whoCanCreateProjects !== undefined)
      settings['who_can_create_projects'] = s.whoCanCreateProjects;
    if (s.whoCanCreateTeams !== undefined) settings['who_can_create_teams'] = s.whoCanCreateTeams;
    if (s.defaultProjectVisibility !== undefined)
      settings['default_project_visibility'] = s.defaultProjectVisibility;
    if (s.require2fa !== undefined) settings['require_2fa'] = s.require2fa;
    if (s.hoursPerDay !== undefined) settings['hours_per_day'] = s.hoursPerDay;
    if (s.daysPerWeek !== undefined) settings['days_per_week'] = s.daysPerWeek;
    if (s.timezone !== undefined) settings['timezone'] = s.timezone;
    body['settings'] = settings;
  }
  return body;
}

/** Context for logo reads: a missing logo is an expected answer, so no toast and no loading bar. */
function logoReadContext(): HttpContext {
  return new HttpContext().set(SKIP_ERROR_NOTIFICATION, true).set(SKIP_LOADING, true);
}

/** Organization endpoints: list, create, settings, members, invitations, logo and public invitations. */
@Injectable({ providedIn: 'root' })
export class OrganizationApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(): Observable<Organization[]> {
    return this.http
      .get<{ data: OrganizationDto[] }>(`${this.baseUrl}/organizations`)
      .pipe(map((response) => response.data.map(toOrganization)));
  }

  /** Creates an organization; the caller becomes its owner. */
  create(input: CreateOrganizationInput): Observable<Organization> {
    const body: Record<string, string> = { name: input.name };
    if (input.slug) body['slug'] = input.slug;
    if (input.description) body['description'] = input.description;
    return this.http
      .post<{ data: OrganizationDto }>(`${this.baseUrl}/organizations`, body)
      .pipe(map((response) => toOrganization(response.data)));
  }

  /** Removes the caller's membership. The last owner cannot leave (org.last_owner). */
  leave(organizationId: string): Observable<void> {
    return this.http.post<void>(`${this.orgUrl(organizationId)}/leave`, null);
  }

  /** Creates one invitation per item in a single request. Plain tokens are returned once. */
  createInvitations(
    organizationId: string,
    items: readonly InvitationItemInput[],
  ): Observable<InvitationCreated[]> {
    return this.http
      .post<{ data: { invitations: InvitationCreatedDto[] } }>(
        `${this.orgUrl(organizationId)}/invitations`,
        {
          invitations: items.map((item) => ({ email: item.email, role: item.role })),
        },
      )
      .pipe(map((response) => response.data.invitations.map(toInvitationCreated)));
  }

  /** Public. Unknown tokens fail with org.invitation_not_found. */
  previewInvitation(token: string): Observable<InvitationPreview> {
    return this.http
      .get<{ data: InvitationPreviewDto }>(
        `${this.baseUrl}/invitations/${encodeURIComponent(token)}`,
        {
          context: publicRequest(),
        },
      )
      .pipe(map((response) => toPreview(response.data)));
  }

  /** Joins the organization of the invitation with the caller's account. */
  acceptInvitation(token: string): Observable<OrganizationMembership> {
    return this.http
      .post<{ data: MembershipDto }>(
        `${this.baseUrl}/invitations/${encodeURIComponent(token)}/accept`,
        null,
      )
      .pipe(map((response) => toMembership(response.data)));
  }

  updateOrganization(organizationId: string, patch: OrganizationPatch): Observable<Organization> {
    return this.http
      .patch<{ data: OrganizationDto }>(this.orgUrl(organizationId), toPatchBody(patch))
      .pipe(map((response) => toOrganization(response.data)));
  }

  /** Uploads the logo as multipart. Resolves with the logo URL reported by the API. */
  uploadLogo(organizationId: string, file: File): Observable<string> {
    const form = new FormData();
    form.append('file', file);
    return this.http
      .put<{ data: { logo_url: string } }>(`${this.orgUrl(organizationId)}/logo`, form)
      .pipe(map((response) => response.data.logo_url));
  }

  /**
   * Logo as a blob. The img element cannot send the Authorization header, so the caller builds an
   * object URL from the blob. Resolves with null when the organization has no logo (404).
   */
  getLogo(organizationId: string): Observable<Blob | null> {
    return this.http
      .get(`${this.orgUrl(organizationId)}/logo`, {
        responseType: 'blob',
        context: logoReadContext(),
      })
      .pipe(
        catchError((error: unknown) => {
          const apiError = asApiError(error);
          return apiError.status === 404 ? of(null) : throwError(() => apiError);
        }),
      );
  }

  /** Logo blob for the logo tiles: any failure resolves with null. */
  fetchLogoBlob(organizationId: string): Observable<Blob | null> {
    return this.getLogo(organizationId).pipe(catchError(() => of(null)));
  }

  deleteLogo(organizationId: string): Observable<void> {
    return this.http.delete<void>(`${this.orgUrl(organizationId)}/logo`);
  }

  listMembers(
    organizationId: string,
    options: { readonly silent?: boolean } = {},
  ): Observable<OrganizationMember[]> {
    return this.http
      .get<{ data: OrganizationMemberDto[] }>(`${this.orgUrl(organizationId)}/members`, {
        context: options.silent ? silentRequest() : undefined,
      })
      .pipe(map((response) => response.data.map(toMember)));
  }

  changeMemberRole(organizationId: string, userId: string, role: OrgRole): Observable<void> {
    return this.http.patch<void>(
      `${this.orgUrl(organizationId)}/members/${encodeURIComponent(userId)}`,
      { role },
    );
  }

  removeMember(organizationId: string, userId: string): Observable<void> {
    return this.http.delete<void>(
      `${this.orgUrl(organizationId)}/members/${encodeURIComponent(userId)}`,
    );
  }

  transferOwnership(organizationId: string, userId: string): Observable<void> {
    return this.http.post<void>(`${this.orgUrl(organizationId)}/transfer-ownership`, {
      user_id: userId,
    });
  }

  /** The slug must be repeated in confirmSlug (org.confirmation_mismatch otherwise). Owners only. */
  deleteOrganization(organizationId: string, confirmSlug: string): Observable<void> {
    return this.http.delete<void>(this.orgUrl(organizationId), {
      body: { confirm_slug: confirmSlug },
    });
  }

  listInvitations(organizationId: string, scope: InvitationScope): Observable<Invitation[]> {
    return this.http
      .get<{ data: InvitationDto[] }>(`${this.orgUrl(organizationId)}/invitations`, {
        params: { status: scope },
      })
      .pipe(map((response) => response.data.map(toInvitation)));
  }

  revokeInvitation(organizationId: string, invitationId: string): Observable<void> {
    return this.http.delete<void>(
      `${this.orgUrl(organizationId)}/invitations/${encodeURIComponent(invitationId)}`,
    );
  }

  /** Returns the new plain token and accept URL; the previous link stops working. */
  resendInvitation(organizationId: string, invitationId: string): Observable<InvitationCreated> {
    return this.http
      .post<{ data: InvitationCreatedDto }>(
        `${this.orgUrl(organizationId)}/invitations/${encodeURIComponent(invitationId)}/resend`,
        null,
      )
      .pipe(map((response) => toInvitationCreated(response.data)));
  }

  private orgUrl(organizationId: string): string {
    return `${this.baseUrl}/organizations/${encodeURIComponent(organizationId)}`;
  }
}
