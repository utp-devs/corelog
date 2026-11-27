// Package defaults holds the drafted Spanish legal documents the platform starts with. They are templates
// with placeholders in square brackets and must be reviewed by a legal advisor before publication.
package defaults

import (
	"embed"

	"github.com/jhonsferg/corelog/internal/legal/domain"
)

//go:embed texts/*.md
var texts embed.FS

// Document is a default document at version 1.
type Document struct {
	Slug     domain.Slug
	Title    string
	Summary  string
	Required bool
	Body     string
}

type spec struct {
	slug     domain.Slug
	file     string
	title    string
	summary  string
	required bool
}

var specs = []spec{
	{domain.SlugPrivacyPolicy, "privacy_policy.md", "Política de privacidad", "Qué datos recopilamos, para qué los usamos, con quién los compartimos y qué derechos tiene.", true},
	{domain.SlugDataProcessing, "data_processing.md", "Política de tratamiento de datos personales", "Finalidades, base legal, plazos de conservación, destinatarios y cómo ejercer los derechos de acceso, rectificación, cancelación y oposición.", true},
	{domain.SlugTermsOfUse, "terms_of_use.md", "Términos de uso", "Condiciones para usar la plataforma: cuenta, uso aceptable, propiedad, disponibilidad, responsabilidad y terminación.", true},
	{domain.SlugCookies, "cookies.md", "Política de cookies", "Cookies y almacenamiento local que usa la plataforma y cómo gestionarlos.", false},
	{domain.SlugCommunications, "communications.md", "Autorización para comunicaciones del producto", "Autorización opcional para recibir novedades del producto por correo y mensaje de texto.", false},
}

// Documents returns the default documents in display order.
func Documents() []Document {
	out := make([]Document, 0, len(specs))
	for _, s := range specs {
		body, err := texts.ReadFile("texts/" + s.file)
		if err != nil {
			panic("legal defaults: missing embedded text " + s.file)
		}
		out = append(out, Document{Slug: s.slug, Title: s.title, Summary: s.summary, Required: s.required, Body: string(body)})
	}
	return out
}
