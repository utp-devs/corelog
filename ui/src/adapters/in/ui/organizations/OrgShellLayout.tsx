import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { AppLayout } from '@/adapters/in/ui/layout/AppLayout'

import { OrgSwitcher } from './OrgSwitcher'

/** App shell fed with the active organization and its switcher. Used by the router under RequireOrg. */
export function OrgShellLayout() {
  const { org, role } = useCurrentOrg()
  const info = org ? { id: org.id, name: org.name, slug: org.slug, role } : null
  return <AppLayout org={info} orgSwitcherSlot={org ? <OrgSwitcher /> : null} />
}
