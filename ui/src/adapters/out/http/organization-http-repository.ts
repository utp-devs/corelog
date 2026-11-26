import type {
  CreateOrganizationInput,
  InvitationItemInput,
  OrganizationRepositoryPort,
} from '@/core/organizations/application/ports/organization-repository.port'
import type {
  Invitation,
  InvitationCreated,
  InvitationPreview,
  InvitationStatus,
  Organization,
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

function toMembership(dto: MembershipDto): OrganizationMembership {
  return {
    organizationId: dto.organization_id,
    userId: dto.user_id,
    role: dto.role,
    joinedAt: dto.joined_at,
  }
}

export class OrganizationHttpRepository implements OrganizationRepositoryPort {
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
}
