import type {
  InvitationScope,
  OrganizationAdminRepositoryPort,
  OrganizationUpdate,
} from '@/core/organizations/application/ports/organization-admin-repository.port'
import type {
  CreateOrganizationInput,
  InvitationItemInput,
} from '@/core/organizations/application/ports/organization-repository.port'
import type {
  Invitation,
  InvitationCreated,
  InvitationPreview,
  InvitationStatus,
  Organization,
  OrganizationMember,
  OrganizationMembership,
  OrganizationSettings,
  OrgRole,
} from '@/core/organizations/domain/organization'
import { apiClient } from './api-client'

interface SettingsDto {
  who_can_create_projects: OrganizationSettings['whoCanCreateProjects']
  who_can_create_teams: OrganizationSettings['whoCanCreateTeams']
  default_project_visibility: OrganizationSettings['defaultProjectVisibility']
  require_2fa: boolean
  max_upload_bytes: number
  hours_per_day: number
  days_per_week: number
  timezone: string
}

interface OrganizationDto {
  id: string
  slug: string
  name: string
  description: string
  settings: SettingsDto
  role?: OrgRole
  has_logo?: boolean
  created_at: string
  updated_at: string
}

interface InvitationDto {
  id: string
  email: string
  role: OrgRole
  status: InvitationStatus
  invited_by: string | null
  expires_at: string
  accepted_at: string | null
  revoked_at: string | null
  created_at: string
}

interface InvitationCreatedDto extends InvitationDto {
  token: string
  accept_url: string
}

interface InvitationPreviewDto {
  organization_name: string
  role: OrgRole
  email: string
  status: InvitationStatus
  expires_at: string
}

interface OrganizationMemberDto {
  user_id: string
  name: string
  display_name?: string
  email: string
  job_title?: string
  department?: string
  has_avatar?: boolean
  role: OrgRole
  joined_at: string
}

interface MembershipDto {
  organization_id: string
  user_id: string
  role: OrgRole
  joined_at: string
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
  }
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
  }
}

function toInvitationCreated(dto: InvitationCreatedDto): InvitationCreated {
  return { ...toInvitation(dto), token: dto.token, acceptUrl: dto.accept_url }
}

function toPreview(dto: InvitationPreviewDto): InvitationPreview {
  return {
    organizationName: dto.organization_name,
    role: dto.role,
    email: dto.email,
    status: dto.status,
    expiresAt: dto.expires_at,
  }
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
  }
}

/** Sends only the fields present in the update, in snake_case (PATCH is a partial update). */
function toUpdateBody(update: OrganizationUpdate): Record<string, unknown> {
  const body: Record<string, unknown> = {}
  if (update.name !== undefined) body.name = update.name
  if (update.description !== undefined) body.description = update.description
  if (update.settings) {
    const s = update.settings
    const settings: Record<string, unknown> = {}
    if (s.whoCanCreateProjects !== undefined) settings.who_can_create_projects = s.whoCanCreateProjects
    if (s.whoCanCreateTeams !== undefined) settings.who_can_create_teams = s.whoCanCreateTeams
    if (s.defaultProjectVisibility !== undefined) settings.default_project_visibility = s.defaultProjectVisibility
    if (s.require2fa !== undefined) settings.require_2fa = s.require2fa
    if (s.hoursPerDay !== undefined) settings.hours_per_day = s.hoursPerDay
    if (s.daysPerWeek !== undefined) settings.days_per_week = s.daysPerWeek
    if (s.timezone !== undefined) settings.timezone = s.timezone
    body.settings = settings
  }
  return body
}

function toMembership(dto: MembershipDto): OrganizationMembership {
  return {
    organizationId: dto.organization_id,
    userId: dto.user_id,
    role: dto.role,
    joinedAt: dto.joined_at,
  }
}

export class OrganizationHttpRepository implements OrganizationAdminRepositoryPort {
  async list(): Promise<Organization[]> {
    const { data } = await apiClient.get<{ data: OrganizationDto[] }>('/organizations')
    return data.data.map(toOrganization)
  }

  async create(input: CreateOrganizationInput): Promise<Organization> {
    const body: Record<string, string> = { name: input.name }
    if (input.slug) body.slug = input.slug
    if (input.description) body.description = input.description
    const { data } = await apiClient.post<{ data: OrganizationDto }>('/organizations', body)
    return toOrganization(data.data)
  }

  async leave(organizationId: string): Promise<void> {
    await apiClient.post(`/organizations/${encodeURIComponent(organizationId)}/leave`)
  }

  async createInvitations(organizationId: string, items: InvitationItemInput[]): Promise<InvitationCreated[]> {
    const { data } = await apiClient.post<{ data: { invitations: InvitationCreatedDto[] } }>(
      `/organizations/${encodeURIComponent(organizationId)}/invitations`,
      { invitations: items.map((item) => ({ email: item.email, role: item.role })) },
    )
    return data.data.invitations.map(toInvitationCreated)
  }

  async previewInvitation(token: string): Promise<InvitationPreview> {
    const { data } = await apiClient.get<{ data: InvitationPreviewDto }>(`/invitations/${encodeURIComponent(token)}`)
    return toPreview(data.data)
  }

  async acceptInvitation(token: string): Promise<OrganizationMembership> {
    const { data } = await apiClient.post<{ data: MembershipDto }>(`/invitations/${encodeURIComponent(token)}/accept`)
    return toMembership(data.data)
  }

  async updateOrganization(organizationId: string, update: OrganizationUpdate): Promise<Organization> {
    const { data } = await apiClient.patch<{ data: OrganizationDto }>(
      `/organizations/${encodeURIComponent(organizationId)}`,
      toUpdateBody(update),
    )
    return toOrganization(data.data)
  }

  async uploadLogo(organizationId: string, file: File): Promise<string> {
    const form = new FormData()
    form.append('file', file)
    const { data } = await apiClient.put<{ data: { logo_url: string } }>(
      `/organizations/${encodeURIComponent(organizationId)}/logo`,
      form,
      { headers: { 'Content-Type': 'multipart/form-data' } },
    )
    return data.data.logo_url
  }

  async getLogo(organizationId: string): Promise<Blob | null> {
    try {
      // responseType blob: <img> cannot send the Authorization header, so the caller builds an object URL.
      const { data } = await apiClient.get<Blob>(`/organizations/${encodeURIComponent(organizationId)}/logo`, {
        responseType: 'blob',
      })
      return data
    } catch (error) {
      if ((error as { status?: number }).status === 404) return null
      throw error
    }
  }

  async deleteLogo(organizationId: string): Promise<void> {
    await apiClient.delete(`/organizations/${encodeURIComponent(organizationId)}/logo`)
  }

  async listMembers(organizationId: string): Promise<OrganizationMember[]> {
    const { data } = await apiClient.get<{ data: OrganizationMemberDto[] }>(
      `/organizations/${encodeURIComponent(organizationId)}/members`,
    )
    return data.data.map(toMember)
  }

  async changeMemberRole(organizationId: string, userId: string, role: OrgRole): Promise<void> {
    await apiClient.patch(
      `/organizations/${encodeURIComponent(organizationId)}/members/${encodeURIComponent(userId)}`,
      { role },
    )
  }

  async removeMember(organizationId: string, userId: string): Promise<void> {
    await apiClient.delete(
      `/organizations/${encodeURIComponent(organizationId)}/members/${encodeURIComponent(userId)}`,
    )
  }

  async transferOwnership(organizationId: string, userId: string): Promise<void> {
    await apiClient.post(`/organizations/${encodeURIComponent(organizationId)}/transfer-ownership`, {
      user_id: userId,
    })
  }

  async deleteOrganization(organizationId: string, confirmSlug: string): Promise<void> {
    await apiClient.delete(`/organizations/${encodeURIComponent(organizationId)}`, {
      data: { confirm_slug: confirmSlug },
    })
  }

  async listInvitations(organizationId: string, scope: InvitationScope): Promise<Invitation[]> {
    const { data } = await apiClient.get<{ data: InvitationDto[] }>(
      `/organizations/${encodeURIComponent(organizationId)}/invitations`,
      { params: { status: scope } },
    )
    return data.data.map(toInvitation)
  }

  async revokeInvitation(organizationId: string, invitationId: string): Promise<void> {
    await apiClient.delete(
      `/organizations/${encodeURIComponent(organizationId)}/invitations/${encodeURIComponent(invitationId)}`,
    )
  }

  async resendInvitation(organizationId: string, invitationId: string): Promise<InvitationCreated> {
    const { data } = await apiClient.post<{ data: InvitationCreatedDto }>(
      `/organizations/${encodeURIComponent(organizationId)}/invitations/${encodeURIComponent(invitationId)}/resend`,
    )
    return toInvitationCreated(data.data)
  }
}
