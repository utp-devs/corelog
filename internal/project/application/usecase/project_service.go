package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	authorization "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

const platformAdmin = "admin"

type ProjectService struct {
	repo        port.Repository
	memberRepo  port.MemberRepository
	catalogRepo port.CatalogRepository
	orgs        port.OrgDirectory
	authorizer  port.ProjectAuthorizer
	events      port.EventPublisher
	tx          txRunner
	teamGrants  port.TeamGrantRepository
	recents     port.RecentRecorder
}

// txRunner runs fn inside a transaction. It matches database.TxManager without importing it here.
type txRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

func NewProjectService(repo port.Repository, memberRepo port.MemberRepository, orgs port.OrgDirectory, catalogRepo ...port.CatalogRepository) *ProjectService {
	var catalog port.CatalogRepository
	if len(catalogRepo) > 0 {
		catalog = catalogRepo[0]
	}
	return &ProjectService{repo: repo, memberRepo: memberRepo, orgs: orgs, catalogRepo: catalog}
}

// WithAuthorizer conecta el autorizador de acciones de proyecto. Sin el, el servicio deniega.
func (s *ProjectService) WithAuthorizer(authorizer port.ProjectAuthorizer) *ProjectService {
	s.authorizer = authorizer
	return s
}

// WithEvents conecta el publicador de eventos de proyecto. Sin el, las acciones no publican eventos.
func (s *ProjectService) WithEvents(events port.EventPublisher) *ProjectService {
	s.events = events
	return s
}

// WithTeamGrants conecta el repositorio de roles de equipo. Sin el, los equipos no aportan acceso y
// las rutas de equipos responden como vacias o no validas.
func (s *ProjectService) WithTeamGrants(teamGrants port.TeamGrantRepository) *ProjectService {
	s.teamGrants = teamGrants
	return s
}

// WithRecents conecta el registro de vistas recientes. Los fallos se registran y no afectan a la lectura.
func (s *ProjectService) WithRecents(recents port.RecentRecorder) *ProjectService {
	s.recents = recents
	return s
}

// WithTransactions conecta el gestor de transacciones. Sin el, cada escritura va sin transaccion
// envolvente.
func (s *ProjectService) WithTransactions(tx txRunner) *ProjectService {
	s.tx = tx
	return s
}

var (
	_ port.Service        = (*ProjectService)(nil)
	_ port.ProjectManager = (*ProjectService)(nil)
)

// Create crea un proyecto de tipo legado (kanban o scrum) en la organizacion.
func (s *ProjectService) Create(ctx context.Context, actorID, orgID uuid.UUID, key, name, description string, projectType domain.ProjectType, visibility domain.Visibility) (*domain.Project, error) {
	if projectType != domain.ProjectTypeKanban && projectType != domain.ProjectTypeScrum {
		return nil, domain.ErrInvalidProject
	}
	return s.CreateProject(ctx, actorID, orgID, port.CreateInput{
		Key:         key,
		Name:        name,
		Description: description,
		Template:    domain.TemplateFromProjectType(projectType),
		Visibility:  visibility,
	})
}

// Get returns a project the actor can see and records the view as a recent item.
func (s *ProjectService) Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error) {
	project, _, err := s.loadVisible(ctx, actorID, globalRole, id)
	if err != nil {
		return nil, err
	}
	if s.recents != nil {
		if err := s.recents.RecordView(ctx, actorID, "project", project.ID); err != nil {
			slog.Warn("failed to record project view", "error", err)
		}
	}
	return project, nil
}

// List devuelve los proyectos visibles para el actor dentro de una organizacion.
func (s *ProjectService) List(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID) ([]*domain.Project, error) {
	orgRole, err := s.orgRoleOf(ctx, orgID, actorID)
	if err != nil {
		return nil, err
	}
	if orgRole == "" && globalRole != platformAdmin {
		return nil, domain.ErrOrganizationNotFound
	}
	return s.repo.FindByOrganization(ctx, orgID, s.scopeFor(actorID, globalRole, orgRole))
}

func (s *ProjectService) Update(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, input domain.UpdateInput) (*domain.Project, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionEditProject, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if err := project.Update(input); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

// Delete es la entrada legada: borra logicamente sin confirmacion escrita. Los llamadores nuevos
// usan DeleteProject.
func (s *ProjectService) Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, id)
	if err != nil {
		return err
	}
	if err := s.requireManage(ctx, port.ActionDeleteProject, project, actorID, globalRole, orgRole); err != nil {
		return err
	}
	return s.softDelete(ctx, project, actorID)
}

// ListMembers returns the direct members followed by the users reached only through team grants.
func (s *ProjectService) ListMembers(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]port.MemberEntry, error) {
	if _, _, err := s.loadVisible(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	direct, err := s.memberRepo.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var teamUsers []port.TeamUserGrant
	if s.teamGrants != nil {
		if teamUsers, err = s.teamGrants.ListTeamUsers(ctx, projectID); err != nil {
			return nil, err
		}
	}
	return mergeMembers(direct, teamUsers), nil
}

func (s *ProjectService) AddMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*port.Member, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionManageMembers, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if err := validateProjectRole(role); err != nil {
		return nil, err
	}
	_, found, err := s.orgs.MemberRole(ctx, project.OrganizationID, userID)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to check organization membership", err)
	}
	if !found {
		return nil, domain.ErrUserNotInOrg
	}
	member := port.Member{
		ProjectID: projectID,
		UserID:    userID,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.memberRepo.Add(ctx, member); err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *ProjectService) UpdateMemberRole(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*port.Member, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionManageMembers, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if err := validateProjectRole(role); err != nil {
		return nil, err
	}
	if role != string(authorization.ProjectRoleLead) {
		if err := s.guardLastLead(ctx, projectID, userID); err != nil {
			return nil, err
		}
	}
	if err := s.memberRepo.UpdateRole(ctx, projectID, userID, role); err != nil {
		return nil, err
	}
	return &port.Member{ProjectID: projectID, UserID: userID, Role: role}, nil
}

func (s *ProjectService) RemoveMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID) error {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return err
	}
	if err := s.requireManage(ctx, port.ActionManageMembers, project, actorID, globalRole, orgRole); err != nil {
		return err
	}
	if err := s.guardLastLead(ctx, projectID, userID); err != nil {
		return err
	}
	return s.memberRepo.Remove(ctx, projectID, userID)
}

func (s *ProjectService) ListLabels(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Label, error) {
	if _, _, err := s.loadVisible(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if s.catalogRepo == nil {
		return []domain.Label{}, nil
	}
	return s.catalogRepo.ListLabels(ctx, projectID)
}

func (s *ProjectService) CreateLabel(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, color string) (*domain.Label, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionEditIssue, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if s.catalogRepo == nil {
		return nil, domain.ErrInvalidProject
	}
	label, err := domain.NewLabel(projectID, name, color)
	if err != nil {
		return nil, err
	}
	if err := s.catalogRepo.CreateLabel(ctx, label); err != nil {
		return nil, err
	}
	return label, nil
}

func (s *ProjectService) DeleteLabel(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, labelID uuid.UUID) error {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return err
	}
	if err := s.requireManage(ctx, port.ActionEditIssue, project, actorID, globalRole, orgRole); err != nil {
		return err
	}
	if s.catalogRepo == nil {
		return domain.ErrInvalidProject
	}
	return s.catalogRepo.DeleteLabel(ctx, projectID, labelID)
}

func (s *ProjectService) ListComponents(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Component, error) {
	if _, _, err := s.loadVisible(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if s.catalogRepo == nil {
		return []domain.Component{}, nil
	}
	return s.catalogRepo.ListComponents(ctx, projectID)
}

func (s *ProjectService) CreateComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, description string) (*domain.Component, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionEditIssue, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if s.catalogRepo == nil {
		return nil, domain.ErrInvalidProject
	}
	component, err := domain.NewComponent(projectID, name, description)
	if err != nil {
		return nil, err
	}
	if err := s.catalogRepo.CreateComponent(ctx, component); err != nil {
		return nil, err
	}
	return component, nil
}

func (s *ProjectService) DeleteComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, componentID uuid.UUID) error {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return err
	}
	if err := s.requireManage(ctx, port.ActionEditIssue, project, actorID, globalRole, orgRole); err != nil {
		return err
	}
	if s.catalogRepo == nil {
		return domain.ErrInvalidProject
	}
	return s.catalogRepo.DeleteComponent(ctx, projectID, componentID)
}

// scopeFor builds the listing scope of an actor: platform admins and org owners/admins see every
// project; org members also see open projects; everyone else sees explicit memberships only.
func (s *ProjectService) scopeFor(actorID uuid.UUID, globalRole, orgRole string) port.VisibilityScope {
	return port.VisibilityScope{
		UserID: actorID,
		All:    globalRole == platformAdmin || isOrgManager(orgRole),
		Open:   orgRole == domain.OrgRoleMember,
	}
}

// orgRoleOf devuelve el rol del actor en la organizacion, o "" si no pertenece a ella.
func (s *ProjectService) orgRoleOf(ctx context.Context, orgID, actorID uuid.UUID) (string, error) {
	role, found, err := s.orgs.MemberRole(ctx, orgID, actorID)
	if err != nil {
		return "", apperror.Wrap(apperror.KindInternal, "failed to check organization membership", err)
	}
	if !found {
		return "", nil
	}
	return role, nil
}

// loadVisible carga el proyecto y comprueba que el actor puede verlo. Lo que el actor no puede
// ver responde NotFound, sin revelar que existe.
func (s *ProjectService) loadVisible(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, string, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	orgRole, err := s.orgRoleOf(ctx, project.OrganizationID, actorID)
	if err != nil {
		return nil, "", err
	}
	ok, err := s.canView(ctx, project, actorID, globalRole, orgRole)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", domain.ErrProjectNotFound
	}
	return project, orgRole, nil
}

// canView: platform admin and org owner/admin see everything; members see open projects;
// everything else needs an explicit project membership, checked by the authorizer.
func (s *ProjectService) canView(ctx context.Context, project *domain.Project, actorID uuid.UUID, globalRole, orgRole string) (bool, error) {
	if globalRole == platformAdmin || isOrgManager(orgRole) {
		return true, nil
	}
	if orgRole == "" {
		return false, nil
	}
	if orgRole == domain.OrgRoleMember && project.Visibility == domain.VisibilityOpen {
		return true, nil
	}
	return s.allowed(ctx, port.ActionViewProject, project.ID, actorID, globalRole)
}

// requireManage: org owner/admin are implicit project leads; other actors need the project action.
func (s *ProjectService) requireManage(ctx context.Context, action string, project *domain.Project, actorID uuid.UUID, globalRole, orgRole string) error {
	if globalRole == platformAdmin || isOrgManager(orgRole) {
		// Managers skip the permission scheme, but the archive rules still apply to them.
		if project.IsArchived() && !authorization.ArchivedActionAllowed(action) {
			return authorization.ErrProjectArchived
		}
		return nil
	}
	ok, err := s.allowed(ctx, action, project.ID, actorID, globalRole)
	if err != nil {
		return err
	}
	if !ok {
		return forbidden()
	}
	return nil
}

func isOrgManager(orgRole string) bool {
	return orgRole == domain.OrgRoleOwner || orgRole == domain.OrgRoleAdmin
}

func validateProjectRole(role string) error {
	switch authorization.ProjectRole(role) {
	case authorization.ProjectRoleLead, authorization.ProjectRoleContributor, authorization.ProjectRoleViewer:
		return nil
	default:
		return apperror.New(apperror.KindValidation, "rol de proyecto no valido")
	}
}

func forbidden() error {
	return apperror.New(apperror.KindForbidden, "no tienes permiso para realizar esta operacion sobre el proyecto")
}

// authorize delega la decision de acceso al autorizador. Sin autorizador se deniega.
func (s *ProjectService) authorize(ctx context.Context, action string, projectID, actorID uuid.UUID, globalRole string) error {
	if s.authorizer == nil {
		return forbidden()
	}
	return s.authorizer.Authorize(ctx, action, actorID, globalRole, projectID)
}

// allowed indica si la accion esta permitida; una denegacion no es un error para el llamador.
func (s *ProjectService) allowed(ctx context.Context, action string, projectID, actorID uuid.UUID, globalRole string) (bool, error) {
	err := s.authorize(ctx, action, projectID, actorID, globalRole)
	if err == nil {
		return true, nil
	}
	if apperror.KindOf(err) == apperror.KindForbidden {
		return false, nil
	}
	return false, err
}
