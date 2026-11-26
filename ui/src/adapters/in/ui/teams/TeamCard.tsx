import { Link } from 'react-router-dom'

import { Lozenge } from '@/shared/ui/Lozenge'
import type { Team } from '@/core/teams/domain/team'
import { TeamTile } from './TeamTile'
import styles from './TeamsPage.module.css'

interface TeamCardProps {
  team: Team
}

/** Member count text in Spanish; a team without a count from the API shows zero members. */
function memberCountText(count: number | undefined): string {
  const value = count ?? 0
  return value === 1 ? '1 miembro' : `${value} miembros`
}

/** Card of a team in the list. The whole card is the link to the team page. */
export function TeamCard({ team }: TeamCardProps) {
  const isMaintainer = team.myRole === 'maintainer'
  return (
    <Link to={`/teams/${team.id}`} className={styles.card} data-testid="team-card">
      <div className={styles.cardHeader}>
        <TeamTile id={team.id} name={team.name} color={team.avatarColor} size="lg" />
        <span className={styles.cardName}>{team.name}</span>
        {isMaintainer && <Lozenge tone="info">Responsable</Lozenge>}
      </div>
      <p className={styles.cardDescription}>{team.description || 'Sin descripción'}</p>
      <span className={styles.cardMeta}>{memberCountText(team.memberCount)}</span>
    </Link>
  )
}
