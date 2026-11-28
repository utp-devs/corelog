package email

import (
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
)

func TestNoticeGolden(t *testing.T) {
	got := NewRenderer().Notice(port.Email{
		Event: domain.EventIssueTransitioned, Greeting: "Luis", IssueKey: "WEB-7", Summary: "Pagina rota", ActorName: "Ana",
		Detail: "Abierta -> Hecha", Link: "https://app.example.test/browse/WEB-7", Preference: "https://app.example.test/profile/notifications",
	})
	if got.Subject != "[WEB-7] Ana cambio el estado de una incidencia: Pagina rota" {
		t.Fatalf("subject %q", got.Subject)
	}
	wantText := "Hola Luis,\n\nAna cambio el estado de una incidencia\nWEB-7 Pagina rota\nAbierta -> Hecha\n\n" +
		"Ver en CoreLog: https://app.example.test/browse/WEB-7\n" +
		"\n--\nRecibes este correo por tus preferencias de notificaciones. Puedes cambiarlas en: https://app.example.test/profile/notifications\n"
	if got.Text != wantText {
		t.Fatalf("text:\n%q\nwant:\n%q", got.Text, wantText)
	}
	for _, fragment := range []string{
		`<html lang="es">`, "Hola Luis,", "<strong>Ana cambio el estado de una incidencia</strong>", ">WEB-7</span> Pagina rota",
		"Abierta -&gt; Hecha", `href="https://app.example.test/browse/WEB-7"`, "Ver en CoreLog", `href="https://app.example.test/profile/notifications"`,
	} {
		if !strings.Contains(got.HTML, fragment) {
			t.Errorf("HTML lacks %q", fragment)
		}
	}
}

func TestEveryEventRendersAKeyedSubject(t *testing.T) {
	for _, event := range domain.AllEvents {
		out := NewRenderer().Notice(port.Email{Event: event, IssueKey: "K-1", Summary: "s", ActorName: "Ana", Link: "https://x.test/browse/K-1", Preference: "https://x.test/p"})
		if !strings.HasPrefix(out.Subject, "[K-1] ") || out.Text == "" || out.HTML == "" {
			t.Errorf("%s: %+v", event, out.Subject)
		}
	}
}

func TestUserControlledValuesAreEscapedAndNeverBreakHeaders(t *testing.T) {
	out := NewRenderer().Notice(port.Email{
		Event: domain.EventIssueCommented, Greeting: `<script>alert(1)</script>`, IssueKey: "WEB-1",
		Summary: "Hola\r\nBcc: victima@example.test <img src=x onerror=alert(1)>", ActorName: "Ana\nBcc: x@y.z", Detail: `"quoted" & <b>bold</b>`,
		Link: "javascript:alert(1)", Preference: "https://x.test/p",
	})
	if strings.ContainsAny(out.Subject, "\r\n") {
		t.Fatalf("subject has a line break: %q", out.Subject)
	}
	for _, bad := range []string{"<script>", "<img", "<b>bold</b>", "javascript:"} {
		if strings.Contains(out.HTML, bad) {
			t.Errorf("HTML contains unescaped %q", bad)
		}
	}
	for _, good := range []string{"&lt;script&gt;", "&lt;img src=x onerror=alert(1)&gt;", "&amp;", "&lt;b&gt;bold&lt;/b&gt;"} {
		if !strings.Contains(out.HTML, good) {
			t.Errorf("HTML lacks escaped %q", good)
		}
	}
}

func TestDigestAndFilterMessages(t *testing.T) {
	lines := []port.EmailLine{
		{Label: "WEB-1", Text: "Uno <i>x</i>", Detail: "Ana creo una incidencia", Link: "https://x.test/browse/WEB-1"},
		{Label: "WEB-2", Text: "Dos", Link: "https://x.test/browse/WEB-2"},
	}
	digest := NewRenderer().Digest(port.Email{Greeting: "Luis", Lines: lines, Total: 5, Preference: "https://x.test/p"})
	if digest.Subject != "Resumen diario: 5 novedades" || !strings.Contains(digest.Text, "* WEB-1 - Uno <i>x</i> (Ana creo una incidencia)") || !strings.Contains(digest.Text, "... y 3 mas") {
		t.Fatalf("digest text:\n%s", digest.Text)
	}
	if strings.Contains(digest.HTML, "<i>x</i>") || !strings.Contains(digest.HTML, "y 3 mas") {
		t.Fatal("digest HTML not escaped or missing the remainder")
	}
	one := NewRenderer().Digest(port.Email{Lines: lines[:1], Total: 1})
	if one.Subject != "Resumen diario: 1 novedad" || !strings.HasPrefix(one.Text, "Hola,") {
		t.Fatalf("single digest %q", one.Subject)
	}

	filter := NewRenderer().FilterSubscription(port.Email{Greeting: "Luis", FilterName: "Mis abiertas", Lines: lines, Total: 2, Link: "https://x.test/issues?filter=1", Preference: "https://x.test/p"})
	if filter.Subject != `Filtro "Mis abiertas": 2 incidencias` || !strings.Contains(filter.HTML, "Abrir el filtro") {
		t.Fatalf("filter subject %q", filter.Subject)
	}
	single := NewRenderer().FilterSubscription(port.Email{FilterName: "F", Lines: lines[:1], Total: 1})
	if single.Subject != `Filtro "F": 1 incidencia` {
		t.Fatalf("single filter %q", single.Subject)
	}
}
