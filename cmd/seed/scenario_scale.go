package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

func init() { Register(scaleScenario{}) }

// scaleScenario seeds the demo organization plus extra accounts (one per 1000
// issues) and --issues issues with their comments, labels and watchers. It is
// meant for performance checks, so it skips links, sprints and notifications.
type scaleScenario struct{}

func (scaleScenario) Name() string { return "scale" }

func (scaleScenario) Run(ctx context.Context, db *sqlx.DB, opts Options) error {
	if opts.Issues <= 0 {
		return errors.New("--issues debe ser mayor que 0")
	}
	return persist(ctx, db, buildScale(opts.PasswordHash, opts.Issues))
}

func buildScale(passwordHash string, issues int) *dataset {
	ds := newDataset()
	rng := newRNG(streamScale)

	entries := demoRoster()
	for k := 1; k <= issues/1000; k++ {
		orgRole := "guest"
		if k%5 == 0 {
			orgRole = "member"
		}
		entries = append(entries, demoEntry{
			name:    fmt.Sprintf("Usuario de escala %03d", k),
			email:   fmt.Sprintf("escala%03d@corelog.test", k),
			orgRole: orgRole,
			team:    1 + k%len(demoTeams),
		})
	}

	org := buildDemoOrg(ds, entries, passwordHash)
	emitIssues(ds, generateIssues(rng, org, issues))
	return ds
}
