package domain

type WorkflowTransition struct {
	FromStatus   Status
	ToStatus     Status
	AllowedRoles []string
}
