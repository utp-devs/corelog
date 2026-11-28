package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// perfScale is the size of the generated dataset of the performance checks.
type perfScale struct {
	Projects int
	Issues   int
}

var defaultPerfScale = perfScale{Projects: 40, Issues: 20000}

// perfData describes the generated dataset.
type perfData struct {
	Big       string // the project with the most issues
	BigIssue  string // an issue with a long history
	Dashboard string
	Gadget    string
}

// buildPerfDataset fills the fixture's organization with projects and issues, comments, activity, notifications,
// audit rows, worklogs, status history and calendar events using set based SQL, then analyzes the tables so the
// planner works with real statistics.
func (f *authzFixture) buildPerfDataset(t *testing.T, scale perfScale) perfData {
	t.Helper()
	org := f.orgA
	lead := f.users[actLead]

	// Projects through the API, so every one has its workflow, board and catalogs.
	have := 3
	for i := 0; have < scale.Projects; i++ {
		id := f.create("perfproject", actOrgOwner, http.MethodPost, "/projects", map[string]any{
			"organization_id": org, "key": fmt.Sprintf("PF%02d", i), "name": fmt.Sprintf("Perf %02d", i),
			"template": "scrum", "type": "scrum", "visibility": "open",
		})
		if id == "" {
			t.Fatal("perf: project not created")
		}
		have++
	}
	small := f.create("perfsmall", actOrgOwner, http.MethodPost, "/projects", map[string]any{
		"organization_id": org, "key": "PFSM", "name": "Perf small", "template": "scrum", "type": "scrum", "visibility": "open",
	})
	f.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'lead') ON CONFLICT DO NOTHING`, small, lead)
	f.exec(`INSERT INTO project_members (project_id, user_id, role)
		SELECT p.id, $2, 'lead' FROM projects p WHERE p.organization_id = $1 AND p.deleted_at IS NULL AND p.archived_at IS NULL
		ON CONFLICT DO NOTHING`, org, lead)
	for i := 0; i < 3; i++ {
		if f.create("issue", actLead, http.MethodPost, "/issues", issueBody(small, fmt.Sprintf("Perf pequena %d", i))) == "" {
			t.Fatal("perf: small project issue not created")
		}
	}

	// 8 active sprints in the first projects.
	f.exec(`INSERT INTO sprints (project_id, name, status, start_date, end_date, started_at)
		SELECT id, 'Perf sprint', 'active', current_date - 3, current_date + 11, now() - interval '3 days'
		FROM (SELECT id FROM projects WHERE organization_id = $1 AND deleted_at IS NULL AND archived_at IS NULL AND id <> $2 ORDER BY id LIMIT 8) s`, org, small)

	start := time.Now()
	f.exec(`
	WITH ps AS (
		SELECT id, row_number() OVER (ORDER BY id) AS n, count(*) OVER () AS total
		FROM projects WHERE organization_id = $1 AND deleted_at IS NULL AND archived_at IS NULL AND id <> $2::uuid
	), st AS (
		SELECT (SELECT id FROM statuses WHERE organization_id = $1 AND name = 'Por hacer') AS todo,
		       (SELECT id FROM statuses WHERE organization_id = $1 AND name = 'En curso') AS doing,
		       (SELECT id FROM statuses WHERE organization_id = $1 AND name = 'En revisión') AS review,
		       (SELECT id FROM statuses WHERE organization_id = $1 AND name = 'Hecho') AS done
	), sp AS (
		SELECT project_id, id AS sprint_id FROM sprints WHERE status = 'active'
	)
	INSERT INTO issues (summary, description, status, priority, reporter_id, assignee_id, project_id, issue_type, rank, key_number,
		issue_type_id, priority_id, status_id, created_at, updated_at, story_points, sprint_id, due_date, resolved_at, closed_at)
	SELECT 'Perf incidencia ' || g || ' lorem ipsum',
		'Descripcion de la incidencia ' || g || ' con texto de relleno para la busqueda lorem token' || g,
		CASE g % 4 WHEN 0 THEN 'open' WHEN 1 THEN 'in_progress' WHEN 2 THEN 'in_progress' ELSE 'resolved' END,
		(ARRAY['low','medium','high','critical'])[1 + g % 4],
		$3::uuid, CASE WHEN g % 5 = 0 THEN NULL ELSE $3::uuid END,
		ps.id, (ARRAY['task','bug','story'])[1 + g % 3], 'm' || lpad(g::text, 8, '0'), 1000 + g,
		(SELECT id FROM issue_types WHERE organization_id = $1 AND legacy_key = (ARRAY['task','bug','story'])[1 + g % 3] ORDER BY position LIMIT 1),
		(SELECT id FROM priorities WHERE organization_id = $1 AND legacy_key = (ARRAY['low','medium','high','critical'])[1 + g % 4] ORDER BY position LIMIT 1),
		CASE g % 4 WHEN 0 THEN st.todo WHEN 1 THEN st.doing WHEN 2 THEN st.review ELSE st.done END,
		now() - (g % 120) * interval '1 day' - (g % 24) * interval '1 hour',
		now() - (g % 60) * interval '1 day',
		(g % 8)::numeric, CASE WHEN g % 3 = 0 THEN sp.sprint_id END, current_date + (g % 30 - 10),
		CASE WHEN g % 4 = 3 THEN now() - (g % 30) * interval '1 day' END,
		CASE WHEN g % 4 = 3 THEN now() - (g % 30) * interval '1 day' END
	FROM generate_series(1, $4::int) g
	JOIN ps ON ps.n = 1 + g % ps.total
	CROSS JOIN st
	LEFT JOIN sp ON sp.project_id = ps.id`, org, small, lead, scale.Issues)
	t.Logf("perf: issues inserted in %s", time.Since(start).Round(time.Millisecond))

	f.exec(`INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at, transition_id)
		SELECT i.id, NULL, (SELECT id FROM statuses WHERE organization_id = $1 AND name = 'Por hacer'), $2, i.created_at, NULL
		FROM issues i WHERE i.key_number > 1000`, org, lead)
	f.exec(`INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at, transition_id)
		SELECT i.id, (SELECT id FROM statuses WHERE organization_id = $1 AND name = 'Por hacer'), i.status_id, $2, i.created_at + interval '2 days', 'start'
		FROM issues i WHERE i.key_number > 1000 AND i.status_id <> (SELECT id FROM statuses WHERE organization_id = $1 AND name = 'Por hacer')`, org, lead)
	f.exec(`INSERT INTO activities (issue_id, actor_id, action, field_name, old_value, new_value, created_at)
		SELECT i.id, $1, 'updated', 'status', 'open', i.status, i.updated_at FROM issues i WHERE i.key_number > 1000`, lead)
	f.exec(`INSERT INTO comments (issue_id, author_id, body, created_at, updated_at)
		SELECT i.id, $1, 'Comentario de prueba ' || i.key_number, i.created_at, i.created_at FROM issues i WHERE i.key_number > 1000 AND i.key_number % 2 = 0`, lead)
	f.exec(`INSERT INTO worklogs (issue_id, author_id, started_at, time_spent_seconds, description, created_at, updated_at)
		SELECT i.id, $1, i.created_at, 3600, 'trabajo', i.created_at, i.created_at FROM issues i WHERE i.key_number > 1000 AND i.key_number % 2 = 1`, lead)

	// One issue with a long history and a long discussion.
	var big, bigIssue string
	if err := f.db.Get(&big, `SELECT project_id::text FROM issues WHERE key_number > 1000 GROUP BY project_id ORDER BY count(*) DESC, project_id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Get(&bigIssue, `SELECT id::text FROM issues WHERE project_id = $1 AND key_number > 1000 ORDER BY key_number LIMIT 1`, big); err != nil {
		t.Fatal(err)
	}
	f.exec(`INSERT INTO activities (issue_id, actor_id, action, field_name, old_value, new_value, created_at)
		SELECT $1, $2, 'updated', 'summary', 'a' || g, 'b' || g, now() - g * interval '1 minute' FROM generate_series(1, 400) g`, bigIssue, lead)
	f.exec(`INSERT INTO comments (issue_id, author_id, body, created_at, updated_at)
		SELECT $1, $2, 'Hilo ' || g, now() - g * interval '1 minute', now() - g * interval '1 minute' FROM generate_series(1, 200) g`, bigIssue, lead)

	f.exec(`INSERT INTO notifications (user_id, kind, message, link, created_at, read_at)
		SELECT $1, 'issue_assigned', 'Te asignaron la incidencia ' || g, '/issues/' || g, now() - g * interval '1 minute', CASE WHEN g % 3 = 0 THEN now() END
		FROM generate_series(1, $2) g`, lead, scale.Issues)
	f.exec(`INSERT INTO notifications (user_id, kind, message, link, created_at)
		SELECT $1, 'issue_assigned', 'Ruido de otro usuario ' || g, '/x', now() FROM generate_series(1, $2) g`, f.users[actMember], scale.Issues/4)
	f.exec(`INSERT INTO audit_events (organization_id, project_id, actor_id, action, entity_type, entity_id, summary, created_at)
		SELECT $1, i.project_id, $2, 'issue.updated', 'issue', i.id, 'Se actualizo la incidencia', i.updated_at FROM issues i WHERE i.key_number > 1000`, org, lead)
	f.exec(`INSERT INTO calendar_events (owner_id, organization_id, project_id, title, starts_at, ends_at, visibility, color)
		SELECT $1, $2, NULL, 'Evento ' || g, now() + (g % 90 - 45) * interval '1 day' + (g % 12) * interval '1 hour',
			now() + (g % 90 - 45) * interval '1 day' + (g % 12 + 1) * interval '1 hour', 'private', 'blue'
		FROM generate_series(1, 1500) g`, lead, org)

	// A dashboard with a statistics gadget for the dashboard check.
	dash := f.create("perfdash", actLead, http.MethodPost, "/dashboards", map[string]any{"name": "Perf panel"})
	var gadget string
	if dash != "" {
		status, raw := f.call(http.MethodPut, "/dashboards/"+dash+"/gadgets", f.tokens[actLead], map[string]any{"gadgets": []any{
			map[string]any{"type": "issue_statistics", "config": map[string]any{"group_by": "status"}, "column_index": 0},
		}})
		if status != http.StatusOK {
			t.Fatalf("perf: gadgets: %d %s", status, truncate(raw, 200))
		}
		_ = f.db.Get(&gadget, `SELECT id::text FROM dashboard_gadgets WHERE dashboard_id = $1 LIMIT 1`, dash)
	}

	for _, table := range []string{"issues", "activities", "comments", "worklogs", "issue_status_history", "notifications", "audit_events", "calendar_events", "sprints", "projects"} {
		f.exec(`VACUUM (ANALYZE) ` + table)
	}
	return perfData{Big: big, BigIssue: bigIssue, Dashboard: dash, Gadget: gadget}
}

// growPerfDataset adds rows to everything the hot endpoints read for the caller (issues of the big project, history of
// the big issue, notifications, calendar events, audit rows) without changing the shape of the data.
func (f *authzFixture) growPerfDataset(t *testing.T, data perfData, round int) {
	t.Helper()
	lead := f.users[actLead]
	f.exec(`INSERT INTO issues (summary, description, status, priority, reporter_id, assignee_id, project_id, issue_type, rank, key_number,
			issue_type_id, priority_id, status_id, created_at, updated_at, story_points, sprint_id, due_date)
		SELECT summary, description, status, priority, reporter_id, assignee_id, project_id, issue_type, rank || 'x', key_number + 500000 + $2::int * 10000 + row_number() OVER (),
			issue_type_id, priority_id, status_id, created_at, updated_at, story_points, sprint_id, due_date
		FROM issues WHERE project_id = $1 AND key_number BETWEEN 1001 AND 400000 LIMIT 100`, data.Big, round)
	f.exec(`INSERT INTO activities (issue_id, actor_id, action, field_name, old_value, new_value, created_at)
		SELECT $1, $2, 'updated', 'summary', 'c' || g, 'd' || g, now() - (g + 1000) * interval '1 minute' FROM generate_series(1, 200) g`, data.BigIssue, lead)
	f.exec(`INSERT INTO comments (issue_id, author_id, body, created_at, updated_at)
		SELECT $1, $2, 'Mas hilo ' || g, now() - (g + 1000) * interval '1 minute', now() FROM generate_series(1, 100) g`, data.BigIssue, lead)
	f.exec(`INSERT INTO notifications (user_id, kind, message, link, created_at)
		SELECT $1, 'issue_assigned', 'Otra ' || g, '/x', now() - (g + 100000) * interval '1 minute' FROM generate_series(1, 1000) g`, lead)
	f.exec(`INSERT INTO calendar_events (owner_id, organization_id, title, starts_at, ends_at, visibility, color)
		SELECT $1, $2, 'Mas ' || g, now() + (g % 40 - 20) * interval '1 day', now() + (g % 40 - 20) * interval '1 day' + interval '1 hour', 'private', 'blue'
		FROM generate_series(1, 100) g`, lead, f.orgA)
	f.exec(`INSERT INTO audit_events (organization_id, actor_id, action, entity_type, summary, created_at)
		SELECT $1, $2, 'issue.updated', 'issue', 'Mas', now() - (g + 1000) * interval '1 minute' FROM generate_series(1, 500) g`, f.orgA, lead)
}
