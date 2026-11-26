import type {
  InvitationCreated,
  InvitationPreview,
  Organization,
  OrganizationMembership,
  OrgRole,
} from '@/core/organizations/domain/organization'

export interface CreateOrganizationInput {
  name: string
  slug?: string
  description?: string
}

export interface InvitationItemInput {
  email: string
  role: OrgRole
}

export interface OrganizationRepositoryPort {
  /** Organizations of the caller, each with the caller's role. */
  list(): Promise<Organization[]>
  /** Creates an organization; the caller becomes its owner. */
  create(input: CreateOrganizationInput): Promise<Organization>
  /** Removes the caller's membership. The last owner cannot leave (org.last_owner). */
  leave(organizationId: string): Promise<void>
  /** Creates one invitation per item, atomically. Plain tokens are returned once. */
  createInvitations(organizationId: string, items: InvitationItemInput[]): Promise<InvitationCreated[]>
  /** Public. Unknown tokens reject with org.invitation_not_found. */
  previewInvitation(token: string): Promise<InvitationPreview>
  /** Joins the organization of the invitation with the caller's account. */
  acceptInvitation(token: string): Promise<OrganizationMembership>
}

