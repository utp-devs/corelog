package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// PermissionService gestiona el esquema de permisos de un proyecto. Un proyecto que no puede
// ver el usuario responde como inexistente (404), nunca como denegado.
type PermissionService struct {
	authorizer *Authorizer
	schemes    port.SchemeRepository
}

// NewPermissionService combina el autorizador (que ya consulta los esquemas) con el repositorio
// de escritura.
func NewPermissionService(authorizer *Authorizer, schemes port.SchemeRepository) *PermissionService {
	return &PermissionService{authorizer: authorizer, schemes: schemes}
}

// SchemeView es el esquema completo de un proyecto. IsDefault indica que no tiene filas propias.
type SchemeView struct {
	ProjectID uuid.UUID
	IsDefault bool
	Grants    []domain.Grant
}

// EffectiveView son los permisos que el usuario tiene en el proyecto. Role es "none" sin rol.
type EffectiveView struct {
	Role        string
	Permissions []domain.Permission
}

// Get devuelve el esquema del proyecto. Requiere poder ver el proyecto.
func (s *PermissionService) Get(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (SchemeView, error) {
	if err := s.requireBrowse(ctx, actorID, globalRole, projectID); err != nil {
		return SchemeView{}, err
	}
	stored, err := s.schemes.Load(ctx, projectID)
	if err != nil {
		return SchemeView{}, err
	}
	if stored == nil {
		return defaultView(projectID), nil
	}
	return schemeView(projectID, *stored, false), nil
}

// Replace valida y guarda un esquema completo. Requiere edit_project.
func (s *PermissionService) Replace(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, grants []domain.Grant) (SchemeView, error) {
	if err := s.requireEdit(ctx, actorID, globalRole, projectID); err != nil {
		return SchemeView{}, err
	}
	scheme, err := domain.NewScheme(grants)
	if err != nil {
		return SchemeView{}, err
	}
	if err := s.schemes.Replace(ctx, projectID, scheme); err != nil {
		return SchemeView{}, err
	}
	return schemeView(projectID, scheme, false), nil
}

// Reset borra las filas propias del proyecto, que vuelve al esquema por defecto. Requiere edit_project.
func (s *PermissionService) Reset(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (SchemeView, error) {
	if err := s.requireEdit(ctx, actorID, globalRole, projectID); err != nil {
		return SchemeView{}, err
	}
	if err := s.schemes.Reset(ctx, projectID); err != nil {
		return SchemeView{}, err
	}
	return defaultView(projectID), nil
}

// Effective devuelve el rol del usuario y los permisos que tiene en el proyecto. Requiere poder ver el proyecto.
func (s *PermissionService) Effective(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (EffectiveView, error) {
	if err := s.requireBrowse(ctx, actorID, globalRole, projectID); err != nil {
		return EffectiveView{}, err
	}
	role, granted, err := s.authorizer.Effective(ctx, actorID, globalRole, projectID)
	if err != nil {
		return EffectiveView{}, err
	}
	view := EffectiveView{Role: "none", Permissions: granted}
	if role != nil {
		view.Role = string(*role)
	}
	return view, nil
}

func (s *PermissionService) requireBrowse(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	ok, err := s.authorizer.Can(ctx, actorID, globalRole, projectID, domain.PermissionBrowseProject)
	if err != nil {
		return err
	}
	if !ok {
		return projectNotFound()
	}
	return nil
}

func (s *PermissionService) requireEdit(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	if err := s.requireBrowse(ctx, actorID, globalRole, projectID); err != nil {
		return err
	}
	return s.authorizer.Require(ctx, actorID, globalRole, projectID, domain.PermissionEditProject)
}

func defaultView(projectID uuid.UUID) SchemeView {
	return schemeView(projectID, domain.DefaultScheme(), true)
}

func schemeView(projectID uuid.UUID, scheme domain.Scheme, isDefault bool) SchemeView {
	return SchemeView{ProjectID: projectID, IsDefault: isDefault, Grants: scheme.Grants()}
}

// projectNotFound uses the same code as the project module so clients handle both alike.
func projectNotFound() error {
	return apperror.WithCode(apperror.KindNotFound, "project.not_found", "proyecto no encontrado")
}
