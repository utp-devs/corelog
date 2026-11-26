package main

import (
	"context"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const demoIssueCount = 120

func init() { Register(demoScenario{}) }

// demoScenario reproduces the former scripts/seed-demo.sql: 21 accounts, 5 teams,
// 4 projects, 120 issues with comments, watchers, links, sprints, saved filters
// and notifications. All accounts share the password from SEED_PASSWORD.
type demoScenario struct{}

func (demoScenario) Name() string { return "demo" }

func (demoScenario) Run(ctx context.Context, db *sqlx.DB, opts Options) error {
	return persist(ctx, db, buildDemo(opts.PasswordHash))
}

func buildDemo(passwordHash string) *dataset {
	ds := newDataset()
	rng := newRNG(streamDemo)
	org := buildOrg(ds, demoOrgSpec(), passwordHash)

	core := org.projects[0]
	sprint11 := addSprint(ds, core.id, core.key, "Sprint 11", "Cierre de la iteracion anterior", "closed",
		daysFromBase(-35), daysFromBase(-21), ptr(daysFromBase(-21)))
	sprint12 := addSprint(ds, core.id, core.key, "Sprint 12", "Estabilizar la API y el tablero", "active",
		daysFromBase(-7), daysFromBase(7), nil)
	addSprint(ds, org.projects[1].id, org.projects[1].key, "Sprint 1", "Primera iteracion de operaciones", "planned",
		daysFromBase(14), daysFromBase(28), nil)
	addBoard(ds, core.id, core.key, "Sprint 12", "kanban", false, catalogCreated)

	recs := generateIssues(rng, org, demoIssueCount)
	assignDemoSprints(recs, core.id, sprint11, sprint12)
	emitIssues(ds, recs)
	addDemoLinks(ds, rng, recs, org.users[0])
	addDemoFilters(ds)
	addDemoNotifications(ds, recs)
	return ds
}

// assignDemoSprints puts part of the CORE issues into the closed and active sprints.
func assignDemoSprints(recs []issueRec, core uuid.UUID, closedSprint, activeSprint uuid.UUID) {
	for idx := range recs {
		r := &recs[idx]
		if r.projectID != core {
			continue
		}
		n := idx + 1
		switch {
		case (r.status == "open" || r.status == "in_progress") && n%3 == 0:
			r.sprint = ptr(activeSprint)
		case (r.status == "resolved" || r.status == "closed") && n%3 == 1:
			r.sprint = ptr(closedSprint)
		}
	}
}

// addDemoLinks links about 3% of the issue pairs of a project, creating the link
// from the older issue to the newer one as the former SQL did.
func addDemoLinks(ds *dataset, rng *rand.Rand, recs []issueRec, admin uuid.UUID) {
	links := ds.table("issue_links", "id", "source_issue_id", "target_issue_id", "kind", "created_at", "created_by")
	kinds := []string{"blocks", "relates", "duplicates"}
	for a := range recs {
		for b := range recs {
			if a == b || recs[a].projectID != recs[b].projectID || !recs[b].created.After(recs[a].created) {
				continue
			}
			if rng.Float64() < 0.03 {
				links.add(idFor("issue_link", recs[a].id.String(), recs[b].id.String()),
					recs[a].id, recs[b].id, kinds[rng.IntN(len(kinds))], baseTime, admin)
			}
		}
	}
}

func addDemoFilters(ds *dataset) {
	filters := ds.table("saved_filters", "id", "owner_id", "name", "filters", "created_at", "updated_at")
	admin := userID("admin@corelog.local")
	agent := userID("agent@corelog.local")
	at := daysFromBase(-30)
	for _, f := range []struct {
		owner         uuid.UUID
		name, payload string
	}{
		{admin, "Criticos abiertos", `{"status":"open","priority":"critical"}`},
		{admin, "En progreso", `{"status":"in_progress"}`},
		{admin, "Bugs", `{"issue_type":"bug"}`},
		{agent, "Mis pendientes", `{"status":"in_progress"}`},
		{agent, "Alta prioridad", `{"priority":"high"}`},
	} {
		filters.add(idFor("saved_filter", f.owner.String(), f.name), f.owner, f.name, f.payload, at, at)
	}
}

// addDemoNotifications gives the admin, the agent and the requester a few
// notifications about existing issues, half of them already read.
func addDemoNotifications(ds *dataset, recs []issueRec) {
	notes := ds.table("notifications", "id", "user_id", "kind", "message", "link", "read_at", "created_at")
	for ri, email := range []string{"admin@corelog.local", "agent@corelog.local", "requester@corelog.local"} {
		uid := userID(email)
		for j := 0; j < 5; j++ {
			rec := recs[(ri*17+j*11)%len(recs)]
			kind, message := "issue_assigned", "Te asignaron "+rec.title
			if j%2 == 1 {
				kind, message = "comment_added", "Nuevo comentario en "+rec.title
			}
			created := daysFromBase(-(j + 1))
			var readAt any
			if j < 2 {
				readAt = created.Add(12 * time.Hour)
			}
			notes.add(idFor("notification", email, strconv.Itoa(j)), uid, kind, message,
				"/issues/"+rec.id.String(), readAt, created)
		}
	}
}
