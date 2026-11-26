import type {
  Invitation,
  InvitationCreated,
  Organization,
  OrganizationMember,
  OrganizationSettings,
  OrgRole,
} from '@/core/organizations/domain/organization'
import type { OrganizationRepositoryPort } from './organization-repository.port'

export type InvitationScope = 'pending' | 'all'

/** Partial update: omitted fields keep their value (PATCH /organizations/{org}). */
export interface OrganizationUpdate {
  name?: string
  description?: string
  settings?: Partial<OrganizationSettings>
}

/** Organization admin screens: settings, members, invitations, logo and danger zone. */
export interface OrganizationAdminRepositoryPort extends OrganizationRepositoryPort {
  updateOrganization(organizationId: string, update: OrganizationUpdate): Promise<Organization>
  uploadLogo(organizationId: string, file: File): Promise<string>
  /** Resolves with null when the organization has no logo (404 org.logo_not_found). */
  getLogo(organizationId: string): Promise<Blob | null>
  deleteLogo(organizationId: string): Promise<void>
  listMembers(organizationId: string): Promise<OrganizationMember[]>
  changeMemberRole(organizationId: string, userId: string, role: OrgRole): Promise<void>
  removeMember(organizationId: string, userId: string): Promise<void>
  transferOwnership(organizationId: string, userId: string): Promise<void>
  /** The organization slug must be repeated in confirmSlug (org.confirmation_mismatch otherwise). */
  deleteOrganization(organizationId: string, confirmSlug: string): Promise<void>
  listInvitations(organizationId: string, scope: InvitationScope): Promise<Invitation[]>
  revokeInvitation(organizationId: string, invitationId: string): Promise<void>
  /** Returns the new plain token and accept URL; the old link stops working. */
  resendInvitation(organizationId: string, invitationId: string): Promise<InvitationCreated>
}
