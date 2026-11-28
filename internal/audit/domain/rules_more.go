package domain

// fixed builds a rule whose summary is the actor followed by a fixed Spanish phrase.
func fixed(entityType, phrase string) Rule {
	return Rule{EntityType: entityType, Summary: func(a string, _ map[string]any) string { return a + " " + phrase }}
}

// keyed builds a rule whose summary names the issue key when the payload has one.
func keyed(entityType, phrase string) Rule {
	return Rule{EntityType: entityType, Summary: func(a string, p map[string]any) string {
		if k := str(p, "key"); k != "" {
			return a + " " + phrase + " " + k
		}
		return a + " " + phrase
	}}
}

func init() {
	more := map[string]Rule{
		"project.restored":                {EntityType: "project", Summary: func(a string, p map[string]any) string { return a + " restauró el proyecto" + named(p, "name") }},
		"invitation.resent":               fixed("invitation", "reenvió una invitación a la organización"),
		"invitation.revoked":              fixed("invitation", "revocó una invitación a la organización"),
		"issue.created":                   keyed("issue", "creó la incidencia"),
		"issue.updated":                   keyed("issue", "actualizó la incidencia"),
		"issue.deleted":                   keyed("issue", "eliminó la incidencia"),
		"issue.restored":                  keyed("issue", "restauró la incidencia"),
		"issue.transitioned":              keyed("issue", "cambió el estado de la incidencia"),
		"issue.link_added":                keyed("issue", "añadió un vínculo a la incidencia"),
		"issue.link_removed":              keyed("issue", "quitó un vínculo de la incidencia"),
		"issue.worklogged":                keyed("issue", "registró tiempo en la incidencia"),
		"comment.created":                 fixed("comment", "comentó en una incidencia"),
		"version.released":                {EntityType: "version", Summary: func(a string, p map[string]any) string { return a + " publicó la versión" + named(p, "name") }},
		"sprint.started":                  fixed("sprint", "inició un sprint"),
		"sprint.completed":                fixed("sprint", "completó un sprint"),
		"user.two_factor_enabled":         fixed("user", "activó la autenticación en dos pasos"),
		"user.two_factor_disabled":        fixed("user", "desactivó la autenticación en dos pasos"),
		"user.recovery_code_used":         fixed("user", "usó un código de recuperación"),
		"user.recovery_codes_regenerated": fixed("user", "regeneró sus códigos de recuperación"),
		"legal.consent_recorded":          fixed("user_consent", "aceptó documentos legales"),
		"legal.consent_revoked":           fixed("user_consent", "revocó un consentimiento"),
		"legal.document_published":        {EntityType: "legal_document", Summary: func(a string, p map[string]any) string { return a + " publicó un documento legal" + named(p, "slug") }},
		"auth.password_reset_requested":   fixed("user", "solicitó restablecer su contraseña"),
		"auth.password_reset":             fixed("user", "restableció su contraseña"),
		"auth.password_changed":           fixed("user", "cambió su contraseña"),
		"auth.email_verified":             fixed("user", "verificó su correo electrónico"),
		"auth.account_locked":             fixed("user", "fue bloqueado temporalmente por intentos fallidos"),
	}
	for k, v := range more {
		if _, exists := DefaultRules[k]; !exists {
			DefaultRules[k] = v
		}
	}
}
