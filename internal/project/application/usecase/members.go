package usecase

import (
	"context"

	"github.com/google/uuid"

	authorization "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// roleRank orders project roles so the stronger role has the lower rank.
func roleRank(role string) int {
	switch authorization.ProjectRole(role) {
	case authorization.ProjectRoleLead:
		return 0
	case authorization.ProjectRoleContributor:
		return 1
	case authorization.ProjectRoleViewer:
		return 2
	default:
		return 3
	}
}

// strongerRole returns the role with the lower rank; ties keep a.
func strongerRole(a, b string) string {
	if roleRank(b) < roleRank(a) {
		return b
	}
	return a
}

// mergeMembers builds the member list: every direct member with its effective role (the best of the
// explicit role and the team grants of the user), then the users reached only through teams. Each
// team-only user appears once, with the team that gives the best role.
func mergeMembers(direct []port.Member, teamUsers []port.TeamUserGrant) []port.MemberEntry {
	bestTeam := map[uuid.UUID]port.TeamUserGrant{}
	var order []uuid.UUID
	for _, grant := range teamUsers {
		current, seen := bestTeam[grant.UserID]
		if !seen {
			order = append(order, grant.UserID)
			bestTeam[grant.UserID] = grant
			continue
		}
		if roleRank(grant.Role) < roleRank(current.Role) {
			bestTeam[grant.UserID] = grant
		}
	}

	entries := make([]port.MemberEntry, 0, len(direct)+len(order))
	isDirect := make(map[uuid.UUID]bool, len(direct))
	for _, member := range direct {
		isDirect[member.UserID] = true
		entry := port.MemberEntry{Member: member, EffectiveRole: member.Role}
		if grant, ok := bestTeam[member.UserID]; ok {
			entry.EffectiveRole = strongerRole(member.Role, grant.Role)
		}
		entries = append(entries, entry)
	}
	for _, userID := range order {
		if isDirect[userID] {
			continue
		}
		grant := bestTeam[userID]
		teamID := grant.TeamID
		entries = append(entries, port.MemberEntry{
			Member:        port.Member{UserID: userID, Role: grant.Role},
			EffectiveRole: grant.Role,
			ViaTeam:       true,
			TeamID:        &teamID,
			TeamName:      grant.TeamName,
		})
	}
	return entries
}

// guardLastLead refuses a removal or demotion that would leave the project without a direct lead.
// The target is looked up in the direct memberships; a user who is not a direct member passes and
// the repository reports the missing membership.
func (s *ProjectService) guardLastLead(ctx context.Context, projectID, userID uuid.UUID) error {
	members, err := s.memberRepo.List(ctx, projectID)
	if err != nil {
		return err
	}
	var target *port.Member
	leads := 0
	for i := range members {
		if members[i].Role == string(authorization.ProjectRoleLead) {
			leads++
		}
		if members[i].UserID == userID {
			target = &members[i]
		}
	}
	if target == nil || target.Role != string(authorization.ProjectRoleLead) {
		return nil
	}
	if leads <= 1 {
		return domain.ErrLastLead
	}
	return nil
}
