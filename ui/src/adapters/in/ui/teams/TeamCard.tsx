import { Link } from 'react-router-dom'

import type { Team } from '@/core/teams/domain/team'
import { TeamTile } from './TeamTile'
import styles from './TeamsPage.module.css'

interface TeamCardProps {
  team: Team
}

/** Card of a team in the list. The whole card is the link to the team page. */
export function TeamCard({ team }: TeamCardProps) {
  return (
    <Link to={`/teams/${team.id}`} className={styles.card} data-testid="team-card">
      <div className={styles.cardHeader}>
        <TeamTile id={team.id} name={team.name} color={team.avatarColor} size="lg" />
        <span className={styles.cardName}>{team.name}</span>
      </div>
      <p className={styles.cardDescription}>{team.description || 'Sin descripción'}</p>
    </Link>
  )
}
