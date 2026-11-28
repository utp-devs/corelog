package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	automationdomain "github.com/jhonsferg/corelog/internal/automation/domain"
)

// mentionToken writes a mention in the syntax of the comment editor.
func mentionToken(name string, id uuid.UUID) string {
	return "@[" + name + "](user:" + id.String() + ")"
}

// threadPeople are the users of one seeded comment thread.
type threadPeople struct {
	author, replier, reactor uuid.UUID
	authorName, replierName  string
}

// addCommentThread appends a conversation to an issue: a comment, a reply that mentions the author, an internal
// note for contributors and leads, and reactions to the comments. Positions are relative to the comments the
// issue already has, so the ids stay unique. Comment times follow the issue creation and never pass the base time.
func addCommentThread(rec *issueRec, p threadPeople) {
	first := len(rec.comments)
	at := func(hours int) time.Time {
		return minTime(baseTime, rec.created.Add(day+time.Duration(hours)*time.Hour))
	}
	rec.comments = append(rec.comments,
		commentRec{
			author: p.author, at: at(0),
			body: "Propongo revisar primero el caso con el cliente. Dejo aquí los pasos para reproducirlo:\n\n1. Abrir la incidencia desde el portal.\n2. Repetir la acción con un adjunto grande.",
			reactions: []reactionRec{
				{user: p.replier, emoji: "thumbs_up"}, {user: p.reactor, emoji: "eyes"}, {user: p.reactor, emoji: "rocket"},
			},
		},
		commentRec{
			author: p.replier, at: at(2), parent: ptr(first),
			body:      mentionToken(p.authorName, p.author) + " de acuerdo, lo reproduzco hoy y respondo aquí.",
			mentions:  []uuid.UUID{p.author},
			reactions: []reactionRec{{user: p.author, emoji: "check"}},
		},
		commentRec{
			author: p.author, at: at(4), internalRole: "contributor",
			body:      "Nota interna: el cliente pidió no compartir los detalles del contrato. " + mentionToken(p.replierName, p.replier) + " tenlo en cuenta.",
			mentions:  []uuid.UUID{p.replier},
			reactions: []reactionRec{{user: p.replier, emoji: "heart"}},
		},
	)
}

// addEpicDates gives every epic a start and a due date so the roadmap has bars to draw. The dates only depend on
// the position of the epic, so a repeated seed writes the same values.
func addEpicDates(ds *dataset, recs []issueRec) {
	k := 0
	for _, r := range recs {
		if r.itype != "epic" {
			continue
		}
		start := daysFromBase(-30 + 14*k)
		due := start.AddDate(0, 0, 75)
		ds.runFinally(fmt.Sprintf(`UPDATE issues SET start_date = '%s', due_date = '%s' WHERE id = '%s'`,
			start.Format(time.DateOnly), due.Format(time.DateOnly), r.id))
		k++
	}
}

// componentLeadsSQL names a lead for the first two components of every project: the project lead for the first and
// a contributor for the second. Components that already have a lead are not touched.
const componentLeadsSQL = `
UPDATE components c SET lead_id = p.lead_id
FROM projects p
WHERE p.id = c.project_id AND c.lead_id IS NULL AND p.lead_id IS NOT NULL
  AND c.id = (SELECT c2.id FROM components c2 WHERE c2.project_id = p.id ORDER BY c2.name LIMIT 1);
UPDATE components c SET lead_id = (SELECT m.user_id FROM project_members m
    WHERE m.project_id = c.project_id AND m.role = 'contributor' ORDER BY m.user_id LIMIT 1)
WHERE c.lead_id IS NULL
  AND c.id = (SELECT c2.id FROM components c2 WHERE c2.project_id = c.project_id ORDER BY c2.name OFFSET 1 LIMIT 1)`

// addComponentLeads registers the statement that names the component leads.
func addComponentLeads(ds *dataset) { ds.runFinally(componentLeadsSQL) }

// addVersions seeds, for every project of the organization, a released version "1.0" and an unreleased version
// "1.1" with fix-version issues: the first finished issues of the project fix 1.0, the first open ones fix 1.1,
// and the first bug is reported as affecting 1.0.
func addVersions(ds *dataset, org orgModel, recs []issueRec) {
	versions := ds.ignoreDuplicates("versions", "id", "project_id", "name", "description", "start_date", "release_date",
		"status", "released_at", "created_at", "updated_at")
	links := ds.ignoreDuplicates("issue_versions", "issue_id", "version_id", "kind")
	const perVersion = 3
	for _, proj := range org.projects {
		released := idFor("version", proj.key, "1.0")
		next := idFor("version", proj.key, "1.1")
		releaseDay := daysFromBase(-14)
		versions.add(released, proj.id, "1.0", "Primera versión estable del proyecto.", daysFromBase(-60).Format(time.DateOnly),
			releaseDay.Format(time.DateOnly), "released", releaseDay, catalogCreated, releaseDay)
		versions.add(next, proj.id, "1.1", "Correcciones y mejoras de la versión 1.0.", daysFromBase(-14).Format(time.DateOnly),
			daysFromBase(21).Format(time.DateOnly), "unreleased", nil, catalogCreated, catalogCreated)

		done, open, bugSeen := 0, 0, false
		for _, r := range recs {
			if r.projectID != proj.id || r.itype == "epic" {
				continue
			}
			finished := r.status == "resolved" || r.status == "closed"
			switch {
			case finished && done < perVersion:
				links.add(r.id, released, "fix")
				done++
			case !finished && open < perVersion:
				links.add(r.id, next, "fix")
				open++
			}
			if r.itype == "bug" && !bugSeen {
				links.add(r.id, released, "affects")
				bugSeen = true
			}
		}
	}
}

// demoSchemeRules is the notification scheme override of the main demo project: it widens the default behavior
// (only assignment, mention and reply notices) with watchers and the project lead for a few events, by email too.
func demoSchemeRules() []map[string]any {
	rule := func(event string, channels []string, recipients ...string) map[string]any {
		items := make([]map[string]string, 0, len(recipients))
		for _, r := range recipients {
			items = append(items, map[string]string{"type": r})
		}
		return map[string]any{"event": event, "recipients": items, "channels": channels}
	}
	return []map[string]any{
		rule("issue_commented", []string{"in_app", "email"}, "reporter", "assignee", "watchers"),
		rule("issue_transitioned", []string{"in_app"}, "assignee", "watchers"),
		rule("version_released", []string{"in_app", "email"}, "watchers", "project_lead"),
	}
}

// addDemoNotificationScheme gives the project a notification scheme of its own.
func addDemoNotificationScheme(ds *dataset, org orgModel, project projectModel) {
	raw, err := json.Marshal(demoSchemeRules())
	if err != nil {
		panic(fmt.Sprintf("seed: encode notification scheme: %v", err))
	}
	ds.ignoreDuplicates("notification_schemes", "id", "organization_id", "project_id", "rules", "updated_by", "created_at", "updated_at").
		add(idFor("notification_scheme", project.key), org.orgID, project.id, json.RawMessage(raw), org.users[0], daysFromBase(-20), daysFromBase(-20))
}

// demoAutomationRule is the sample rule of the demo: it is disabled, so it never changes data until someone
// reviews and enables it.
func demoAutomationRule() automationdomain.Rule {
	return automationdomain.Rule{
		Name:    "Avisar al reportero cuando se termina una incidencia",
		Trigger: automationdomain.Trigger{Type: automationdomain.TriggerIssueTransitioned, To: "Hecho"},
		Actions: []automationdomain.Action{
			{Type: automationdomain.ActionSendNotification, Who: automationdomain.WhoReporter, Text: "{{issue.key}} pasó a {{trigger.to}}: {{issue.summary}}"},
			{Type: automationdomain.ActionAddComment, Text: "Regla {{rule.name}}: la incidencia pasó de {{trigger.from}} a {{trigger.to}}."},
		},
	}
}

const (
	demoWebhookURL    = "https://example.com/hooks/corelog"
	demoWebhookSecret = "sembrado: rota el secreto antes de activar el webhook"
)

// demoWebhookEvents are the events of the sample webhook.
var demoWebhookEvents = []string{"issue.created", "issue.transitioned", "version.released"}

// addDemoIntegrations seeds the sample automation rule and the sample webhook of the demo, both disabled. The
// webhook points to a documentation address and holds a placeholder secret that cannot sign anything: rotate the
// secret before enabling it.
func addDemoIntegrations(ds *dataset, org orgModel, project projectModel) {
	rule := demoAutomationRule()
	trigger, _ := json.Marshal(rule.Trigger)
	conditions, _ := json.Marshal([]automationdomain.Condition{})
	actions, _ := json.Marshal(rule.Actions)
	at := daysFromBase(-12)
	ds.ignoreDuplicates("automation_rules", "id", "organization_id", "project_id", "name", "enabled", "trigger", "conditions",
		"actions", "created_by", "version", "run_count", "created_at", "updated_at").
		add(idFor("automation_rule", project.key, rule.Name), org.orgID, project.id, rule.Name, false,
			json.RawMessage(trigger), json.RawMessage(conditions), json.RawMessage(actions), org.users[0], 1, 0, at, at)

	ds.ignoreDuplicates("webhooks", "id", "organization_id", "project_id", "name", "url", "secret_encrypted", "events",
		"filter_query", "enabled", "created_by", "created_at", "updated_at").
		add(idFor("webhook", org.orgID.String(), "Ejemplo de integración"), org.orgID, nil, "Ejemplo de integración",
			demoWebhookURL, []byte(demoWebhookSecret), "{"+strings.Join(demoWebhookEvents, ",")+"}",
			"project = "+strings.ToUpper(project.key), false, org.users[0], at, at)
}
