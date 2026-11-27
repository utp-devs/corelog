package http

import (
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
)

// permissionGrantDTO is one permission with the holders that have it.
type permissionGrantDTO struct {
	Permission string   `json:"permission"`
	Holders    []string `json:"holders"`
}

// permissionSchemeDTO is the full scheme of a project. IsDefault is true when the project has no
// custom rows.
type permissionSchemeDTO struct {
	ProjectID string               `json:"project_id"`
	IsDefault bool                 `json:"is_default"`
	Grants    []permissionGrantDTO `json:"grants"`
}

// replacePermissionsRequest carries the complete scheme. Permissions not listed have no holders.
type replacePermissionsRequest struct {
	Grants []permissionGrantDTO `json:"grants"`
}

// effectivePermissionsDTO is the role of the caller in the project and the permissions it holds.
type effectivePermissionsDTO struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func toPermissionScheme(view authusecase.SchemeView) permissionSchemeDTO {
	grants := make([]permissionGrantDTO, 0, len(view.Grants))
	for _, g := range view.Grants {
		holders := make([]string, 0, len(g.Holders))
		for _, h := range g.Holders {
			holders = append(holders, string(h))
		}
		grants = append(grants, permissionGrantDTO{Permission: string(g.Permission), Holders: holders})
	}
	return permissionSchemeDTO{ProjectID: view.ProjectID.String(), IsDefault: view.IsDefault, Grants: grants}
}
