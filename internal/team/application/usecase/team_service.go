package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/team/application/port"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

// TeamService implements port.Service. Authorization rules:
//   - Visibility: any member of the organization (guests included) may read the teams
//     of that organization. Users outside it get NotFound, so existence is not leaked.
//   - Create: owner and admin always; member only when the org policy is "members";
//     guests never. The creator becomes a team maintainer.
//   - Update, delete and member management: organization owner/admin or a team maintainer.
//     Organization members without that right get Forbidden.
//   - The platform admin role does not bypass organization membership.
type TeamService struct {
	repo port.Repository
	orgs port.OrgDirectory
	tx   database.TxManager
}

func NewTeamService(repo port.Repository, orgs port.OrgDirectory, tx database.TxManager) *TeamService {
	return &TeamService{repo: repo, orgs: orgs, tx: tx}
}

var _ port.Service = (*TeamService)(nil)

func (s *TeamService) Create(ctx context.Context, actorID uuid.UUID, in port.CreateInput) (*domain.Team, error) {
	if in.OrganizationID == uuid.Nil {
		return nil, domain.ErrOrganizationRequired
	}
	if err := s.requireCreator(ctx, in.OrganizationID, actorID); err != nil {
		return nil, err
	}
	if err := s.ensureNameFree(ctx, in.Name, uuid.Nil); err != nil {
		return nil, err
	}

	team, err := domain.NewTeam(domain.NewTeamInput{
		OrganizationID: in.OrganizationID,
		CreatedBy:      actorID,
		Name:           in.Name,
		Description:    in.Description,
		AvatarColor:    in.AvatarColor,
	})
	if err != nil {
		return nil, err
	}

	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Save(ctx, team); err != nil {
			return err
		}
		creator, err := domain.NewMember(team.ID, actorID, domain.RoleMaintainer)
		if err != nil {
			return err
		}
		return s.repo.AddMember(ctx, in.OrganizationID, creator)
	})
	if err != nil {
		return nil, err
	}
	// The creator is the only member, so the read-side values are known without another query.
	team.MemberCount = 1
	team.MyRole = domain.RoleMaintainer
	return team, nil
}

func (s *TeamService) Get(ctx context.Context, actorID, id uuid.UUID) (*domain.Team, error) {
	team, err := s.repo.FindByID(ctx, id, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, team.OrganizationID, actorID); err != nil {
		return nil, err
	}
	return team, nil
}

func (s *TeamService) List(ctx context.Context, actorID, organizationID, memberID uuid.UUID) ([]*domain.Team, error) {
	if organizationID == uuid.Nil {
		return nil, domain.ErrOrganizationRequired
	}
	_, found, err := s.orgRole(ctx, organizationID, actorID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, domain.ErrOrganizationNotFound
	}
	if memberID == uuid.Nil {
		return s.repo.FindByOrganization(ctx, organizationID, actorID)
	}
	return s.repo.FindByOrganizationAndMember(ctx, organizationID, memberID, actorID)
}

func (s *TeamService) Update(ctx context.Context, actorID, id uuid.UUID, patch domain.TeamPatch) (*domain.Team, error) {
	team, err := s.repo.FindByID(ctx, id, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTeamAdmin(ctx, team, actorID); err != nil {
		return nil, err
	}
	if patch.Name != nil {
		if err := s.ensureNameFree(ctx, *patch.Name, team.ID); err != nil {
			return nil, err
		}
	}
	if err := team.Apply(patch); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, team); err != nil {
		return nil, err
	}
	return team, nil
}

func (s *TeamService) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	team, err := s.repo.FindByID(ctx, id, actorID)
	if err != nil {
		return err
	}
	if err := s.requireTeamAdmin(ctx, team, actorID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, team.OrganizationID, team.ID)
}

func (s *TeamService) ListMembers(ctx context.Context, actorID, teamID uuid.UUID) ([]*domain.Member, error) {
	team, err := s.repo.FindByID(ctx, teamID, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, team.OrganizationID, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, team.OrganizationID, team.ID)
}

func (s *TeamService) AddMember(ctx context.Context, actorID, teamID, userID uuid.UUID, role string) (*domain.Member, error) {
	team, err := s.repo.FindByID(ctx, teamID, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTeamAdmin(ctx, team, actorID); err != nil {
		return nil, err
	}
	member, err := domain.NewMember(team.ID, userID, role)
	if err != nil {
		return nil, err
	}
	if err := s.requireOrgMemberTarget(ctx, team.OrganizationID, userID); err != nil {
		return nil, err
	}
	if err := s.repo.AddMember(ctx, team.OrganizationID, member); err != nil {
		return nil, err
	}
	return s.repo.FindMember(ctx, team.OrganizationID, team.ID, userID)
}

func (s *TeamService) UpdateMemberRole(ctx context.Context, actorID, teamID, userID uuid.UUID, role string) (*domain.Member, error) {
	team, err := s.repo.FindByID(ctx, teamID, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTeamAdmin(ctx, team, actorID); err != nil {
		return nil, err
	}
	if !domain.ValidRole(role) {
		return nil, domain.ErrInvalidRole
	}
	if err := s.repo.UpdateMemberRole(ctx, team.OrganizationID, team.ID, userID, role); err != nil {
		return nil, err
	}
	return s.repo.FindMember(ctx, team.OrganizationID, team.ID, userID)
}

func (s *TeamService) RemoveMember(ctx context.Context, actorID, teamID, userID uuid.UUID) error {
	team, err := s.repo.FindByID(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if err := s.requireTeamAdmin(ctx, team, actorID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, team.OrganizationID, team.ID, userID)
}

// orgRole returns the organization role of a user, with found=false for non-members.
func (s *TeamService) orgRole(ctx context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	role, found, err := s.orgs.MemberRole(ctx, orgID, userID)
	if err != nil {
		return "", false, apperror.Wrap(apperror.KindInternal, "failed to resolve organization membership", err)
	}
	return role, found, nil
}

// requireVisible passes for any organization member. Non-members get NotFound.
func (s *TeamService) requireVisible(ctx context.Context, orgID, actorID uuid.UUID) error {
	_, found, err := s.orgRole(ctx, orgID, actorID)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrTeamNotFound
	}
	return nil
}

// requireCreator applies the who_can_create_teams policy of the organization.
func (s *TeamService) requireCreator(ctx context.Context, orgID, actorID uuid.UUID) error {
	role, found, err := s.orgRole(ctx, orgID, actorID)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrOrganizationNotFound
	}
	if domain.IsOrgAdmin(role) {
		return nil
	}
	if role != domain.OrgRoleMember {
		return domain.ErrCreationNotAllowed
	}
	policy, err := s.orgs.TeamCreationPolicy(ctx, orgID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to read organization policy", err)
	}
	if policy != domain.CreationPolicyMembers {
		return domain.ErrCreationNotAllowed
	}
	return nil
}

// requireTeamAdmin passes for organization owner/admin or a team maintainer.
func (s *TeamService) requireTeamAdmin(ctx context.Context, team *domain.Team, actorID uuid.UUID) error {
	role, found, err := s.orgRole(ctx, team.OrganizationID, actorID)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrTeamNotFound
	}
	if domain.IsOrgAdmin(role) {
		return nil
	}
	m, err := s.repo.FindMember(ctx, team.OrganizationID, team.ID, actorID)
	if errors.Is(err, domain.ErrMemberNotFound) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	if m.Role != domain.RoleMaintainer {
		return domain.ErrForbidden
	}
	return nil
}

// requireOrgMemberTarget rejects users that do not belong to the team's organization.
func (s *TeamService) requireOrgMemberTarget(ctx context.Context, orgID, userID uuid.UUID) error {
	_, found, err := s.orgRole(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrUserNotInOrg
	}
	return nil
}

// ensureNameFree returns ErrTeamNameTaken when another team already has the name.
// The schema keeps team names globally unique, so the check is global as well.
func (s *TeamService) ensureNameFree(ctx context.Context, name string, selfID uuid.UUID) error {
	existing, err := s.repo.FindByName(ctx, name)
	if errors.Is(err, domain.ErrTeamNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("team: check name: %w", err)
	}
	if existing.ID != selfID {
		return domain.ErrTeamNameTaken
	}
	return nil
}
