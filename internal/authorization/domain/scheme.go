package domain

import (
	"fmt"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// CodeInvalidScheme is the stable error code of a permission scheme that breaks a rule.
const CodeInvalidScheme = "authorization.invalid_scheme"

// Grant lists the holders that a permission is granted to.
type Grant struct {
	Permission Permission
	Holders    []Holder
}

// Scheme maps every permission to the holders that have it. A permission without holders is
// granted to nobody. Schemes are immutable once built by NewScheme or DefaultScheme.
type Scheme struct {
	holders map[Permission][]Holder
}

// defaultGrants is the scheme used by projects without custom rows. Each entry follows the default
// scheme of the authorization model. delete_project is granted to the lead and to org owners and
// admins (org_admin); deleting still requires typing the project name.
var defaultGrants = map[Permission][]Holder{
	PermissionBrowseProject:       {HolderLead, HolderContributor, HolderViewer},
	PermissionCreateIssue:         {HolderLead, HolderContributor},
	PermissionEditIssue:           {HolderLead, HolderContributor},
	PermissionEditOwnIssue:        {HolderReporter},
	PermissionDeleteIssue:         {HolderLead},
	PermissionAssignIssue:         {HolderLead, HolderContributor},
	PermissionTransitionIssue:     {HolderLead, HolderContributor, HolderAssignee},
	PermissionCommentIssue:        {HolderLead, HolderContributor},
	PermissionEditOwnComment:      {HolderLead, HolderContributor},
	PermissionDeleteOwnComment:    {HolderLead, HolderContributor},
	PermissionManageComments:      {HolderLead},
	PermissionAddAttachment:       {HolderLead, HolderContributor},
	PermissionDeleteOwnAttachment: {HolderLead, HolderContributor},
	PermissionManageAttachments:   {HolderLead},
	PermissionLinkIssue:           {HolderLead, HolderContributor},
	PermissionLogWork:             {HolderLead, HolderContributor},
	PermissionManageWorklogs:      {HolderLead},
	PermissionWatchIssue:          {HolderLead, HolderContributor, HolderViewer},
	PermissionVoteIssue:           {HolderLead, HolderContributor, HolderViewer},
	PermissionMoveCards:           {HolderLead, HolderContributor},
	PermissionManageBoard:         {HolderLead},
	PermissionManageSprints:       {HolderLead},
	PermissionManageVersions:      {HolderLead},
	PermissionManageComponents:    {HolderLead},
	PermissionManageWorkflow:      {HolderLead},
	PermissionManageMembers:       {HolderLead},
	PermissionEditProject:         {HolderLead},
	PermissionDeleteProject:       {HolderLead, HolderOrgAdmin},
	PermissionViewReports:         {HolderLead, HolderContributor, HolderViewer},
	PermissionManageAutomation:    {HolderLead},
	PermissionBulkChange:          {HolderLead, HolderContributor},
	PermissionExportIssues:        {HolderLead, HolderContributor, HolderViewer},
}

// DefaultScheme returns the scheme of a project without custom permission rows.
func DefaultScheme() Scheme {
	holders := make(map[Permission][]Holder, len(defaultGrants))
	for p, hs := range defaultGrants {
		holders[p] = append([]Holder(nil), hs...)
	}
	return Scheme{holders: holders}
}

// NewScheme builds and validates a scheme from grants. Duplicate holders of one permission are
// merged. A permission that is not listed has no holders.
func NewScheme(grants []Grant) (Scheme, error) {
	holders := make(map[Permission][]Holder, len(grants))
	for _, g := range grants {
		if !g.Permission.Valid() {
			return Scheme{}, invalidScheme(fmt.Sprintf("el permiso %q no existe", g.Permission))
		}
		if _, dup := holders[g.Permission]; dup {
			return Scheme{}, invalidScheme(fmt.Sprintf("el permiso %q aparece mas de una vez", g.Permission))
		}
		seen := make(map[Holder]bool, len(g.Holders))
		list := make([]Holder, 0, len(g.Holders))
		for _, h := range g.Holders {
			if !h.Valid() {
				return Scheme{}, invalidScheme(fmt.Sprintf("el titular %q no existe", h))
			}
			if seen[h] {
				continue
			}
			seen[h] = true
			list = append(list, h)
		}
		holders[g.Permission] = list
	}
	scheme := Scheme{holders: holders}
	if err := scheme.Validate(); err != nil {
		return Scheme{}, err
	}
	return scheme, nil
}

// Validate enforces the rules that keep a project manageable:
//   - lead keeps manage_members and edit_project;
//   - delete_project is held by lead or org_admin;
//   - browse_project is held by viewer, contributor or lead.
func (s Scheme) Validate() error {
	if !s.has(PermissionManageMembers, HolderLead) {
		return invalidScheme("el rol lead debe conservar manage_members")
	}
	if !s.has(PermissionEditProject, HolderLead) {
		return invalidScheme("el rol lead debe conservar edit_project")
	}
	if !s.has(PermissionDeleteProject, HolderLead) && !s.has(PermissionDeleteProject, HolderOrgAdmin) {
		return invalidScheme("delete_project debe estar asignado a lead u org_admin")
	}
	if !s.has(PermissionBrowseProject, HolderViewer) &&
		!s.has(PermissionBrowseProject, HolderContributor) &&
		!s.has(PermissionBrowseProject, HolderLead) {
		return invalidScheme("browse_project debe incluir viewer o contributor")
	}
	return nil
}

// Holders returns the holders of a permission, in the order they were stored.
func (s Scheme) Holders(p Permission) []Holder {
	return append([]Holder(nil), s.holders[p]...)
}

// Grants returns one grant per catalog permission, in catalog order.
func (s Scheme) Grants() []Grant {
	out := make([]Grant, 0, len(Permissions))
	for _, p := range Permissions {
		out = append(out, Grant{Permission: p, Holders: s.Holders(p)})
	}
	return out
}

func (s Scheme) has(p Permission, h Holder) bool {
	for _, x := range s.holders[p] {
		if x == h {
			return true
		}
	}
	return false
}

// Allowed is the pure decision: a permission passes when the effective project role is one of its
// holders, or when one of the special holders of the object is listed. role is nil when the caller
// has no role; specials are the holders the caller matches on the object.
func Allowed(scheme Scheme, permission Permission, role *ProjectRole, specials ...Holder) bool {
	for _, h := range scheme.holders[permission] {
		if role != nil && h == Holder(*role) {
			return true
		}
		for _, s := range specials {
			if s == h {
				return true
			}
		}
	}
	return false
}

// Decide applies a scheme to the access context of a caller, using the effective role and the
// organization holders.
func Decide(scheme Scheme, permission Permission, access Access) bool {
	return Allowed(scheme, permission, EffectiveRole(access), SpecialHolders(access)...)
}

func invalidScheme(message string) error {
	return apperror.WithCode(apperror.KindValidation, CodeInvalidScheme, "el esquema de permisos no es valido: "+message)
}
