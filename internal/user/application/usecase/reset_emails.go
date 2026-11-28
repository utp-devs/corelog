package usecase

import (
	"html"
	"net/url"
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/mailer"
)

// resetEmail builds the password reset message. The link carries the token in the query string of the
// frontend route /reset-password.
func resetEmail(to, name, baseURL, token string) mailer.Message {
	link := actionLink(baseURL, "/reset-password", token)
	greeting := greetingFor(name)
	text := greeting + "\n\n" +
		"Recibimos una solicitud para restablecer la contraseña de tu cuenta de CoreLog.\n" +
		"Usa este enlace dentro de la próxima hora:\n\n" + link + "\n\n" +
		"Si no fuiste tú, ignora este mensaje: tu contraseña no cambiará.\n"
	page := "<p>" + html.EscapeString(greeting) + "</p>" +
		"<p>Recibimos una solicitud para restablecer la contraseña de tu cuenta de CoreLog.</p>" +
		"<p><a href=\"" + html.EscapeString(link) + "\">Restablecer mi contraseña</a> (válido durante una hora)</p>" +
		"<p>Si no fuiste tú, ignora este mensaje: tu contraseña no cambiará.</p>"
	return mailer.Message{To: to, Subject: "Restablece tu contraseña de CoreLog", Text: text, HTML: page}
}

// verificationEmail builds the email verification message (frontend route /verify-email).
func verificationEmail(to, name, baseURL, token string) mailer.Message {
	link := actionLink(baseURL, "/verify-email", token)
	greeting := greetingFor(name)
	text := greeting + "\n\n" +
		"Confirma tu correo electrónico para terminar de configurar tu cuenta de CoreLog:\n\n" + link + "\n\n" +
		"El enlace es válido durante 24 horas. Si no creaste la cuenta, ignora este mensaje.\n"
	page := "<p>" + html.EscapeString(greeting) + "</p>" +
		"<p>Confirma tu correo electrónico para terminar de configurar tu cuenta de CoreLog.</p>" +
		"<p><a href=\"" + html.EscapeString(link) + "\">Verificar mi correo</a> (válido durante 24 horas)</p>" +
		"<p>Si no creaste la cuenta, ignora este mensaje.</p>"
	return mailer.Message{To: to, Subject: "Verifica tu correo de CoreLog", Text: text, HTML: page}
}

func greetingFor(name string) string {
	if name = strings.TrimSpace(name); name != "" {
		return "Hola " + name + ","
	}
	return "Hola,"
}

func actionLink(baseURL, path, token string) string {
	return strings.TrimRight(baseURL, "/") + path + "?token=" + url.QueryEscape(token)
}
