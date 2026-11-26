import type { Organization, InvitationPreview, OrganizationMembership } from '@/core/organizations/domain/organization'
import type {
  CreateOrganizationInput,
  InvitationItemInput,
  OrganizationRepositoryPort,
} from './ports/organization-repository.port'

export class OrganizationService {
  private readonly repository: OrganizationRepositoryPort

  constructor(repository: OrganizationRepositoryPort) {
    this.repository = repository
  }

  list(): Promise<Organization[]> {
    return this.repository.list()
  }

  create(input: CreateOrganizationInput): Promise<Organization> {
    return this.repository.create(input)
  }

  leave(organizationId: string): Promise<void> {
    return this.repository.leave(organizationId)
  }

  createInvitations(organizationId: string, items: InvitationItemInput[]) {
    return this.repository.createInvitations(organizationId, items)
  }

  previewInvitation(token: string): Promise<InvitationPreview> {
    return this.repository.previewInvitation(token)
  }

  acceptInvitation(token: string): Promise<OrganizationMembership> {
    return this.repository.acceptInvitation(token)
  }
}
