import type { OrgMembersById, OrgMembersByIdStatus } from '@/adapters/in/ui/organizations/useOrgMembersById'

export const NO_LEAD_LABEL = 'Sin responsable'
export const LEAD_LOADING_LABEL = 'Cargando responsable'

/** Display name of the project lead. Falls back to "Sin responsable" when there is no lead or the name is unknown. */
export function leadNameOf(leadId: string, members: OrgMembersById, status: OrgMembersByIdStatus): string {
  if (status === 'loading') return LEAD_LOADING_LABEL
  const name = leadId ? members.get(leadId)?.name : undefined
  return name && name.trim() ? name : NO_LEAD_LABEL
}
