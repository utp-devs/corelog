package main

import (
	"encoding/json"
	"testing"

	notificationdomain "github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	webhookdomain "github.com/jhonsferg/corelog/internal/webhook/domain"
)

// TestDemoIntegrationsAreValidAndDisabled checks the sample rule, webhook and scheme against the domain rules, so
// the seeded data never needs a fix before someone edits or enables it.
func TestDemoIntegrationsAreValidAndDisabled(t *testing.T) {
	if err := demoAutomationRule().Validate(); err != nil {
		t.Errorf("automation rule: %v", err)
	}
	if _, err := webhookdomain.ValidateURL(demoWebhookURL, false); err != nil {
		t.Errorf("webhook url: %v", err)
	}
	if _, err := webhookdomain.ValidateEvents(demoWebhookEvents); err != nil {
		t.Errorf("webhook events: %v", err)
	}

	raw, err := json.Marshal(demoSchemeRules())
	if err != nil {
		t.Fatal(err)
	}
	var rules []struct {
		Event      string `json:"event"`
		Recipients []struct {
			Type string `json:"type"`
		} `json:"recipients"`
		Channels []string `json:"channels"`
	}
	if err := json.Unmarshal(raw, &rules); err != nil {
		t.Fatal(err)
	}
	scheme := notificationdomain.Scheme{}
	for _, r := range rules {
		rule := notificationdomain.Rule{Event: notificationdomain.Event(r.Event)}
		for _, rec := range r.Recipients {
			rule.Recipients = append(rule.Recipients, notificationdomain.Recipient{Type: notificationdomain.RecipientType(rec.Type)})
		}
		for _, c := range r.Channels {
			rule.Channels = append(rule.Channels, notificationdomain.Channel(c))
		}
		scheme.Rules = append(scheme.Rules, rule)
	}
	if _, err := scheme.Validate(); err != nil {
		t.Errorf("notification scheme: %v", err)
	}
}

func TestE2ECollaborationSeed(t *testing.T) {
	db := testdb.New(t)
	if err := runScenario(t, db, "e2e", testOptions(t)); err != nil {
		t.Fatalf("e2e seed: %v", err)
	}
	positive := map[string]string{
		"released and unreleased versions per project": `SELECT count(*) FROM versions WHERE status = 'released'`,
		"fix-version issues":                           `SELECT count(*) FROM issue_versions WHERE kind = 'fix'`,
		"components with a lead":                       `SELECT count(*) FROM components WHERE lead_id IS NOT NULL`,
		"replies":                                      `SELECT count(*) FROM comments WHERE parent_id IS NOT NULL`,
		"internal comments":                            `SELECT count(*) FROM comments WHERE visibility = 'internal' AND internal_role IS NOT NULL`,
		"mentions":                                     `SELECT count(*) FROM comment_mentions`,
		"reactions":                                    `SELECT count(*) FROM comment_reactions`,
		"worklogs":                                     `SELECT count(*) FROM worklogs`,
		"issues with time spent":                       `SELECT count(*) FROM issues WHERE time_spent_seconds > 0 AND original_estimate_seconds IS NOT NULL`,
		"epics with dates":                             `SELECT count(*) FROM issues WHERE issue_type = 'epic' AND start_date IS NOT NULL AND due_date >= start_date`,
		"closed sprint with committed points":          `SELECT count(*) FROM sprints WHERE status = 'closed' AND committed_points > 0 AND completed_points > 0 AND completed_points < committed_points`,
		"closed sprint verdicts":                       `SELECT count(*) FROM sprint_issue_events WHERE event IN ('completed', 'incomplete')`,
	}
	for name, query := range positive {
		var n int
		if err := db.Get(&n, query); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n == 0 {
			t.Errorf("%s: none seeded", name)
		}
	}
	var versionsPerProject int
	if err := db.Get(&versionsPerProject, `SELECT count(*) FROM (SELECT project_id FROM versions GROUP BY project_id
		HAVING count(*) FILTER (WHERE status = 'released') = 1 AND count(*) FILTER (WHERE status = 'unreleased') = 1) t`); err != nil {
		t.Fatal(err)
	}
	if versionsPerProject != 4 {
		t.Errorf("projects with one released and one unreleased version = %d, want 4", versionsPerProject)
	}
	// The e2e scenario must not create integrations: they could change the data the suite relies on.
	for _, table := range []string{"automation_rules", "webhooks", "notification_schemes"} {
		if got := countRows(t, db, table); got != 0 {
			t.Errorf("%s has %d rows in the e2e scenario, want 0", table, got)
		}
	}
}

func TestDemoCollaborationSeed(t *testing.T) {
	db := testdb.New(t)
	if err := runScenario(t, db, "demo", testOptions(t)); err != nil {
		t.Fatalf("demo seed: %v", err)
	}
	var enabledRules, enabledHooks int
	if err := db.Get(&enabledRules, `SELECT count(*) FROM automation_rules WHERE enabled`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&enabledHooks, `SELECT count(*) FROM webhooks WHERE enabled`); err != nil {
		t.Fatal(err)
	}
	if enabledRules != 0 || enabledHooks != 0 {
		t.Errorf("enabled rules=%d webhooks=%d, want both 0", enabledRules, enabledHooks)
	}
	for table, want := range map[string]int{"automation_rules": 1, "webhooks": 1, "notification_schemes": 1} {
		if got := countRows(t, db, table); got != want {
			t.Errorf("%s = %d, want %d", table, got, want)
		}
	}
	for _, table := range []string{"versions", "issue_versions", "comment_reactions", "comment_mentions", "worklogs"} {
		if countRows(t, db, table) == 0 {
			t.Errorf("no rows in %s", table)
		}
	}
	var internal int
	if err := db.Get(&internal, `SELECT count(*) FROM comments WHERE visibility = 'internal'`); err != nil {
		t.Fatal(err)
	}
	if internal == 0 {
		t.Error("no internal comments in the demo")
	}
}
