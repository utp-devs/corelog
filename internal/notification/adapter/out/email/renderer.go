// Package email renders the Spanish notification emails (plain text and simple HTML). Every user-controlled
// value (summaries, names, details) goes through html/template escaping in the HTML part and is stripped of
// line breaks in headers.
package email

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"unicode"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// Renderer implements port.EmailRenderer.
type Renderer struct{}

var _ port.EmailRenderer = Renderer{}

// NewRenderer returns the renderer.
func NewRenderer() Renderer { return Renderer{} }

// headline is the Spanish sentence of an event, without the issue.
func headline(e port.Email) string {
	actor := strings.TrimSpace(oneLine(e.ActorName))
	if actor == "" {
		actor = "Alguien"
	}
	switch e.Event {
	case domain.EventIssueCreated:
		return actor + " creo una incidencia"
	case domain.EventIssueAssigned:
		return "Te asignaron una incidencia"
	case domain.EventIssueCommented:
		return actor + " comento en una incidencia"
	case domain.EventIssueMentioned:
		return actor + " te menciono en una incidencia"
	case domain.EventIssueTransitioned:
		return actor + " cambio el estado de una incidencia"
	case domain.EventIssueResolved:
		return actor + " resolvio una incidencia"
	case domain.EventIssueUpdated:
		return actor + " actualizo una incidencia"
	case domain.EventCommentEdited:
		return actor + " edito un comentario"
	case domain.EventWorklogLogged:
		return actor + " registro tiempo en una incidencia"
	case domain.EventVersionReleased:
		return actor + " publico una version"
	case domain.EventSprintStarted:
		return actor + " inicio un sprint"
	case domain.EventSprintCompleted:
		return actor + " completo un sprint"
	default:
		return actor + " hizo un cambio"
	}
}

func oneLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsControl(r) }), " ")
}

func subjectOf(e port.Email) string {
	subject := headline(e)
	if e.IssueKey != "" {
		subject = "[" + oneLine(e.IssueKey) + "] " + subject
	}
	if s := oneLine(e.Summary); s != "" {
		subject += ": " + s
	}
	return domain.Truncate(subject, 200)
}

func greeting(name string) string {
	if n := strings.TrimSpace(oneLine(name)); n != "" {
		return "Hola " + n + ","
	}
	return "Hola,"
}

const footerText = "\n--\nRecibes este correo por tus preferencias de notificaciones. Puedes cambiarlas en: "

var layout = template.Must(template.New("layout").Parse(`<!doctype html>
<html lang="es"><body style="margin:0;padding:0;background:#f4f5f7;font-family:Arial,Helvetica,sans-serif;color:#172b4d">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:24px">
<table role="presentation" width="560" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:6px;padding:24px">
<tr><td style="font-size:16px;line-height:24px">
<p style="margin:0 0 12px">{{.Greeting}}</p>
<p style="margin:0 0 12px"><strong>{{.Headline}}</strong></p>
{{if .Summary}}<p style="margin:0 0 12px">{{if .IssueKey}}<span style="color:#5e6c84">{{.IssueKey}}</span> {{end}}{{.Summary}}</p>{{end}}
{{if .Detail}}<p style="margin:0 0 12px;color:#5e6c84">{{.Detail}}</p>{{end}}
{{if .Lines}}<table role="presentation" width="100%" cellpadding="4" cellspacing="0" style="font-size:14px">
{{range .Lines}}<tr><td style="border-top:1px solid #dfe1e6"><a href="{{.Link}}" style="color:#0052cc;text-decoration:none">{{if .Label}}{{.Label}} - {{end}}{{.Text}}</a>{{if .Detail}}<br><span style="color:#5e6c84">{{.Detail}}</span>{{end}}</td></tr>
{{end}}</table>{{if .More}}<p style="margin:12px 0 0;color:#5e6c84">y {{.More}} mas</p>{{end}}{{end}}
{{if .Link}}<p style="margin:16px 0 0"><a href="{{.Link}}" style="background:#0052cc;color:#ffffff;padding:8px 14px;border-radius:4px;text-decoration:none">{{.Button}}</a></p>{{end}}
</td></tr>
<tr><td style="padding-top:20px;font-size:12px;color:#5e6c84">Recibes este correo por tus preferencias de notificaciones. <a href="{{.Preference}}" style="color:#5e6c84">Cambiar preferencias</a></td></tr>
</table></td></tr></table></body></html>`))

type view struct {
	Greeting, Headline, IssueKey, Summary, Detail, Link, Button, Preference string
	Lines                                                                   []port.EmailLine
	More                                                                    int
}

func renderHTML(v view) string {
	var buf bytes.Buffer
	if err := layout.Execute(&buf, v); err != nil {
		// The template is static; a failure here is a programming error, and plain text still goes out.
		return ""
	}
	return buf.String()
}

// Notice renders one event email.
func (Renderer) Notice(e port.Email) port.Rendered {
	head := headline(e)
	var text strings.Builder
	text.WriteString(greeting(e.Greeting) + "\n\n" + head + "\n")
	if e.IssueKey != "" || e.Summary != "" {
		text.WriteString(strings.TrimSpace(oneLine(e.IssueKey)+" "+oneLine(e.Summary)) + "\n")
	}
	if d := oneLine(e.Detail); d != "" {
		text.WriteString(d + "\n")
	}
	if e.Link != "" {
		text.WriteString("\nVer en CoreLog: " + e.Link + "\n")
	}
	text.WriteString(footerText + e.Preference + "\n")
	return port.Rendered{
		Subject: subjectOf(e),
		Text:    text.String(),
		HTML: renderHTML(view{
			Greeting: greeting(e.Greeting), Headline: head, IssueKey: oneLine(e.IssueKey), Summary: oneLine(e.Summary),
			Detail: oneLine(e.Detail), Link: e.Link, Button: "Ver en CoreLog", Preference: e.Preference,
		}),
	}
}

// Digest renders the daily summary.
func (Renderer) Digest(e port.Email) port.Rendered {
	head := fmt.Sprintf("Resumen diario: %d novedades", e.Total)
	if e.Total == 1 {
		head = "Resumen diario: 1 novedad"
	}
	return port.Rendered{
		Subject: head,
		Text:    listText(e, head),
		HTML:    renderHTML(view{Greeting: greeting(e.Greeting), Headline: head, Lines: cleanLines(e.Lines), More: e.Total - len(e.Lines), Preference: e.Preference}),
	}
}

// FilterSubscription renders the results of a subscribed filter.
func (Renderer) FilterSubscription(e port.Email) port.Rendered {
	name := oneLine(e.FilterName)
	head := fmt.Sprintf("Filtro %q: %d incidencias", name, e.Total)
	if e.Total == 1 {
		head = fmt.Sprintf("Filtro %q: 1 incidencia", name)
	}
	return port.Rendered{
		Subject: domain.Truncate(head, 200),
		Text:    listText(e, head),
		HTML: renderHTML(view{
			Greeting: greeting(e.Greeting), Headline: head, Lines: cleanLines(e.Lines), More: e.Total - len(e.Lines),
			Link: e.Link, Button: "Abrir el filtro", Preference: e.Preference,
		}),
	}
}

func cleanLines(lines []port.EmailLine) []port.EmailLine {
	out := make([]port.EmailLine, len(lines))
	for i, l := range lines {
		out[i] = port.EmailLine{Label: oneLine(l.Label), Text: oneLine(l.Text), Detail: oneLine(l.Detail), Link: l.Link}
	}
	return out
}

func listText(e port.Email, head string) string {
	var text strings.Builder
	text.WriteString(greeting(e.Greeting) + "\n\n" + head + "\n\n")
	for _, l := range cleanLines(e.Lines) {
		line := l.Text
		if l.Label != "" {
			line = l.Label + " - " + line
		}
		if l.Detail != "" {
			line += " (" + l.Detail + ")"
		}
		text.WriteString("* " + line + "\n  " + l.Link + "\n")
	}
	if more := e.Total - len(e.Lines); more > 0 {
		text.WriteString(fmt.Sprintf("... y %d mas\n", more))
	}
	text.WriteString(footerText + e.Preference + "\n")
	return text.String()
}
