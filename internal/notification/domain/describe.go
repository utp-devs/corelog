package domain

import (
	"strings"
	"unicode/utf8"
)

// maxMessageRunes is the length limit of a stored notification message.
const maxMessageRunes = 300

// Describe returns the Spanish in-app text of an event. Empty parts are left out.
func Describe(event Event, key, summary, actor, detail string) string {
	who := strings.TrimSpace(actor)
	if who == "" {
		who = "Alguien"
	}
	target := strings.TrimSpace(key)
	if s := strings.TrimSpace(summary); s != "" {
		if target != "" {
			target += ": " + s
		} else {
			target = s
		}
	}
	if target == "" {
		target = "una incidencia"
	}
	var text string
	switch event {
	case EventIssueCreated:
		text = who + " creo " + target
	case EventIssueAssigned:
		text = "Te asignaron la incidencia: " + target
	case EventIssueCommented:
		text = who + " comento en " + target
	case EventIssueMentioned:
		text = who + " te menciono en " + target
	case EventIssueTransitioned:
		text = who + " cambio el estado de " + target
	case EventIssueResolved:
		text = who + " resolvio " + target
	case EventIssueUpdated:
		text = who + " actualizo " + target
	case EventCommentEdited:
		text = who + " edito un comentario en " + target
	case EventWorklogLogged:
		text = who + " registro tiempo en " + target
	case EventVersionReleased:
		text = who + " publico la version"
	case EventSprintStarted:
		text = who + " inicio el sprint"
	case EventSprintCompleted:
		text = who + " completo el sprint"
	case EventFilterSubscription:
		text = "Resultados de tu filtro suscrito"
	default:
		text = who + " hizo un cambio en " + target
	}
	if d := strings.TrimSpace(detail); d != "" {
		text += " (" + d + ")"
	}
	return Truncate(text, maxMessageRunes)
}

// Truncate cuts text to max runes, ending with an ellipsis when it was cut.
func Truncate(text string, max int) string {
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	runes := []rune(text)
	return string(runes[:max-1]) + "…"
}
