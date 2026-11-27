export const NO_LEAD_LABEL = 'Sin responsable';
export const LEAD_LOADING_LABEL = 'Cargando responsable';

export type ProjectLeadMembersStatus = 'idle' | 'loading' | 'ready' | 'error';

/** Organization members indexed by user id; only the name is needed to show the lead. */
export type ProjectLeadMembers = ReadonlyMap<string, { name: string }>;

/** Display name of the project lead. Falls back to "Sin responsable" when there is no lead or the name is unknown. */
export function leadNameOf(
  leadId: string,
  members: ProjectLeadMembers,
  status: ProjectLeadMembersStatus,
): string {
  if (status === 'loading') return LEAD_LOADING_LABEL;
  const name = leadId ? members.get(leadId)?.name : undefined;
  return name && name.trim() ? name : NO_LEAD_LABEL;
}
